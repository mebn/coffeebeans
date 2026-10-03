package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

const maxUploadSize = 15 << 20 // 15 MB

var (
	ollamaURL   = envOr("OLLAMA_URL", "http://localhost:11434")
	ollamaModel = envOr("OLLAMA_MODEL", "qwen3.5:0.8b")
	listenAddr  = envOr("ADDR", ":8080")
)

const systemPrompt = `You are an expert plant pathologist specialising in coffee plants (Coffea arabica and Coffea canephora).
You will receive a photo that should show coffee plant leaves.
Identify any visible diseases, pests or nutrient deficiencies and give a short, simple, practical fix for each.
Common examples: coffee leaf rust (Hemileia vastatrix), cercospora leaf spot (brown eye spot), coffee berry disease, phoma leaf spot, anthracnose, sooty mold, leaf miner damage, nitrogen/potassium/magnesium/iron deficiency.
Rules:
- Keep each "fix" to one or two short sentences a farmer can act on.
- If the leaves look healthy, return an empty "diseases" list and set "healthy" to true.
- If the image does not show coffee leaves, set "is_coffee_leaf" to false and explain briefly in "summary".
- "confidence" must be one of: "low", "medium", "high".
- "summary" is one short sentence.`

// Diagnosis is the JSON shape returned to the frontend and requested from the model.
type Diagnosis struct {
	IsCoffeeLeaf bool      `json:"is_coffee_leaf"`
	Healthy      bool      `json:"healthy"`
	Summary      string    `json:"summary"`
	Diseases     []Disease `json:"diseases"`
}

type Disease struct {
	Name       string `json:"name"`
	Confidence string `json:"confidence"`
	Fix        string `json:"fix"`
}

// JSON schema passed to Ollama's structured output ("format") so the model returns valid JSON.
var diagnosisSchema = json.RawMessage(`{
  "type": "object",
  "properties": {
    "is_coffee_leaf": {"type": "boolean"},
    "healthy": {"type": "boolean"},
    "summary": {"type": "string"},
    "diseases": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "name": {"type": "string"},
          "confidence": {"type": "string", "enum": ["low", "medium", "high"]},
          "fix": {"type": "string"}
        },
        "required": ["name", "confidence", "fix"]
      }
    }
  },
  "required": ["is_coffee_leaf", "healthy", "summary", "diseases"]
}`)

type ollamaMessage struct {
	Role    string   `json:"role"`
	Content string   `json:"content"`
	Images  []string `json:"images,omitempty"`
}

type ollamaChatRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
	Think    bool            `json:"think"`
	Format   json.RawMessage `json:"format"`
	Options  map[string]any  `json:"options,omitempty"`
}

type ollamaChatResponse struct {
	Message ollamaMessage `json:"message"`
	Error   string        `json:"error"`
}

var ollamaClient = &http.Client{Timeout: 5 * time.Minute}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", handleHealth)
	mux.HandleFunc("POST /api/classify", handleClassify)

	log.Printf("listening on %s (model %s at %s)", listenAddr, ollamaModel, ollamaURL)
	log.Fatal(http.ListenAndServe(listenAddr, withCORS(mux)))
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "model": ollamaModel})
}

func handleClassify(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		writeError(w, http.StatusBadRequest, "image too large or invalid form (max 15 MB)")
		return
	}

	file, _, err := r.FormFile("image")
	if err != nil {
		writeError(w, http.StatusBadRequest, `missing "image" file field`)
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		writeError(w, http.StatusBadRequest, "could not read image")
		return
	}

	contentType := http.DetectContentType(data)
	if !strings.HasPrefix(contentType, "image/") {
		writeError(w, http.StatusBadRequest, "file is not an image")
		return
	}

	diagnosis, err := classify(r, data)
	if err != nil {
		log.Printf("classify error: %v", err)
		writeError(w, http.StatusBadGateway, "model request failed: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, diagnosis)
}

func classify(r *http.Request, image []byte) (*Diagnosis, error) {
	reqBody := ollamaChatRequest{
		Model: ollamaModel,
		Messages: []ollamaMessage{
			{Role: "system", Content: systemPrompt},
			{
				Role:    "user",
				Content: "Classify this coffee leaf photo. Return the diagnosis as JSON.",
				Images:  []string{base64.StdEncoding.EncodeToString(image)},
			},
		},
		Stream:  false,
		Think:   false,
		Format:  diagnosisSchema,
		Options: map[string]any{"temperature": 0.2},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, ollamaURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := ollamaClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("is Ollama running? %w", err)
	}
	defer resp.Body.Close()

	var chat ollamaChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chat); err != nil {
		return nil, fmt.Errorf("decode ollama response: %w", err)
	}
	if resp.StatusCode != http.StatusOK || chat.Error != "" {
		return nil, fmt.Errorf("ollama returned %d: %s", resp.StatusCode, chat.Error)
	}

	var diagnosis Diagnosis
	if err := json.Unmarshal([]byte(chat.Message.Content), &diagnosis); err != nil {
		return nil, fmt.Errorf("model returned invalid JSON: %w", err)
	}
	// The model sometimes adds placeholder entries (e.g. "N/A") when there is nothing to report.
	diseases := []Disease{}
	if diagnosis.IsCoffeeLeaf {
		for _, d := range diagnosis.Diseases {
			name := strings.ToLower(strings.TrimSpace(d.Name))
			if name == "" || name == "n/a" || name == "none" || name == "healthy" {
				continue
			}
			diseases = append(diseases, d)
		}
	}
	diagnosis.Diseases = diseases
	if diagnosis.IsCoffeeLeaf && len(diseases) == 0 {
		diagnosis.Healthy = true
	}
	return &diagnosis, nil
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
