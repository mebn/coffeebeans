# Coffee Leaf Doctor

Upload a photo of coffee leaves and get a short list of likely diseases and simple fixes.
Uses a local Ollama vision model (`qwen3.5:0.8b`).

## Run

```sh
# 1. Ollama (must be running, model pulled)
ollama pull qwen3.5:0.8b

# 2. Backend (Go, port 8080)
cd backend && go run .

# 3. Frontend (React + Vite, port 5173, proxies /api to the backend)
cd frontend && npm install && npm run dev
```

Open http://localhost:5173

## Config (backend env vars)

| Var            | Default                  |
| -------------- | ------------------------ |
| `OLLAMA_URL`   | `http://localhost:11434` |
| `OLLAMA_MODEL` | `qwen3.5:0.8b`            |
| `ADDR`         | `:8080`                  |

## API

`POST /api/classify` with multipart field `image`:

```json
{
  "is_coffee_leaf": true,
  "healthy": false,
  "summary": "Orange powdery spots typical of leaf rust.",
  "diseases": [
    { "name": "Coffee leaf rust", "confidence": "high", "fix": "Remove infected leaves and spray a copper-based fungicide." }
  ]
}
```
