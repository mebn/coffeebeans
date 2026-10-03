import { useCallback, useEffect, useRef, useState } from 'react'
import type { ChangeEvent, DragEvent } from 'react'

type Disease = {
  name: string
  confidence: 'low' | 'medium' | 'high'
  fix: string
}

type Diagnosis = {
  is_coffee_leaf: boolean
  healthy: boolean
  summary: string
  diseases: Disease[]
}

export default function App() {
  const [file, setFile] = useState<File | null>(null)
  const [preview, setPreview] = useState<string | null>(null)
  const [dragging, setDragging] = useState(false)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [result, setResult] = useState<Diagnosis | null>(null)
  const inputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    if (!file) return
    const url = URL.createObjectURL(file)
    setPreview(url)
    return () => URL.revokeObjectURL(url)
  }, [file])

  const pickFile = useCallback((f: File | undefined) => {
    if (!f) return
    if (!f.type.startsWith('image/')) {
      setError('Please choose an image file.')
      return
    }
    setFile(f)
    setResult(null)
    setError(null)
  }, [])

  const onDrop = (e: DragEvent<HTMLDivElement>) => {
    e.preventDefault()
    setDragging(false)
    pickFile(e.dataTransfer.files[0])
  }

  const onChange = (e: ChangeEvent<HTMLInputElement>) => {
    pickFile(e.target.files?.[0])
    e.target.value = ''
  }

  const reset = () => {
    setFile(null)
    setPreview(null)
    setResult(null)
    setError(null)
  }

  const analyze = async () => {
    if (!file) return
    setLoading(true)
    setError(null)
    setResult(null)
    try {
      const body = new FormData()
      body.append('image', file)
      const res = await fetch('/api/classify', { method: 'POST', body })
      const data = await res.json()
      if (!res.ok) throw new Error(data.error ?? `Request failed (${res.status})`)
      setResult(data)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Something went wrong')
    } finally {
      setLoading(false)
    }
  }

  return (
    <main className="page">
      <header className="header">
        <div className="logo" aria-hidden>
          <LeafIcon />
        </div>
        <h1>Coffee Leaf Doctor</h1>
        <p className="subtitle">
          Upload a photo of coffee leaves to spot diseases and get simple fixes.
        </p>
      </header>

      <section className="card">
        <div
          className={`dropzone${dragging ? ' dragging' : ''}${preview ? ' has-image' : ''}`}
          onClick={() => !preview && inputRef.current?.click()}
          onDragOver={(e) => {
            e.preventDefault()
            setDragging(true)
          }}
          onDragLeave={() => setDragging(false)}
          onDrop={onDrop}
          role="button"
          tabIndex={0}
          onKeyDown={(e) => {
            if (!preview && (e.key === 'Enter' || e.key === ' ')) inputRef.current?.click()
          }}
        >
          <input ref={inputRef} type="file" accept="image/*" hidden onChange={onChange} />
          {preview ? (
            <>
              <img src={preview} alt="Selected leaf" className="preview" />
              {loading && (
                <div className="scanning">
                  <div className="scan-line" />
                </div>
              )}
            </>
          ) : (
            <div className="dropzone-empty">
              <div className="upload-icon">
                <UploadIcon />
              </div>
              <p className="drop-title">Drop a leaf photo here</p>
              <p className="drop-hint">or click to browse - JPG, PNG, WEBP up to 15 MB</p>
            </div>
          )}
        </div>

        {file && (
          <div className="actions">
            <span className="filename" title={file.name}>
              {file.name}
            </span>
            <div className="buttons">
              <button className="btn ghost" onClick={reset} disabled={loading}>
                Change
              </button>
              <button className="btn primary" onClick={analyze} disabled={loading}>
                {loading ? (
                  <>
                    <span className="spinner" /> Analyzing...
                  </>
                ) : (
                  'Analyze leaf'
                )}
              </button>
            </div>
          </div>
        )}

        {error && <div className="alert">{error}</div>}
      </section>

      {result && <Results result={result} />}

      <footer className="footer">Runs locally with Ollama. Results are guidance, not a lab diagnosis.</footer>
    </main>
  )
}

function Results({ result }: { result: Diagnosis }) {
  const status = !result.is_coffee_leaf ? 'unknown' : result.healthy ? 'healthy' : 'sick'
  const label = {
    unknown: 'Not a coffee leaf',
    healthy: 'Looks healthy',
    sick: `${result.diseases.length} issue${result.diseases.length === 1 ? '' : 's'} found`,
  }[status]

  return (
    <section className="card results">
      <div className="results-head">
        <span className={`badge ${status}`}>{label}</span>
        <p className="summary">{result.summary}</p>
      </div>

      {result.diseases.length > 0 && (
        <ul className="disease-list">
          {result.diseases.map((d, i) => (
            <li key={i} className="disease" style={{ animationDelay: `${i * 80}ms` }}>
              <div className="disease-top">
                <h3>{d.name}</h3>
                <span className={`confidence ${d.confidence}`}>{d.confidence}</span>
              </div>
              <p className="fix">
                <strong>Fix:</strong> {d.fix}
              </p>
            </li>
          ))}
        </ul>
      )}
    </section>
  )
}

function LeafIcon() {
  return (
    <svg viewBox="0 0 24 24" width="28" height="28" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
      <path d="M11 20A7 7 0 0 1 9.8 6.1C15.5 5 17 4.48 19 2c1 2 2 4.18 2 8 0 5.5-4.78 10-10 10Z" />
      <path d="M2 21c0-3 1.85-5.36 5.08-6C9.5 14.52 12 13 13 12" />
    </svg>
  )
}

function UploadIcon() {
  return (
    <svg viewBox="0 0 24 24" width="28" height="28" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
      <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />
      <polyline points="17 8 12 3 7 8" />
      <line x1="12" y1="3" x2="12" y2="15" />
    </svg>
  )
}
