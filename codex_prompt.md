Codex Build Prompt: Archivist MVP
You are helping build Archivist, a capstone project application.
Archivist is a local-first AI workspace for schools. The goal is to let an admin upload course documents, then let students ask questions and receive AI answers grounded in those uploaded documents. This project investigates whether guided workflows can reduce the technical barriers to deploying local AI assistants.
Core MVP Goal
Build the first working foundation of Archivist.
The app should support this workflow:

1. Admin starts the app with Docker Compose.
2. Admin opens the HTMX web GUI.
3. Admin creates or logs into an admin account.
4. Admin creates a course workspace.
5. Admin uploads course documents.
6. Archivist stores and indexes the documents.
7. Student logs in.
8. Student selects an assigned workspace.
9. Student asks questions.
10. Archivist uses a local Ollama model and document context to answer with source references.
    Required Tech Stack
    Use:

- Go for the backend
- SQLite for storage
- HTMX for the web UI
- Go html/template or templ for server-rendered templates
- Bubble Tea for the TUI foundation
- Ollama for local model inference
- Docker Compose for deployment
  Do not use:
- React
- Vue
- Svelte
- GraphQL
- Cloud-only dependencies
- External hosted databases
- OAuth or SSO for the MVP
  Default Local Model Requirements
  Archivist must be able to run in a low-resource Docker setup.
  Use these defaults:
  Default chat model: gemma3:1b
  Default embedding model: nomic-embed-text
  Ollama base URL: http://ollama:11434
  The model should be configurable through environment variables.
  Project Structure
  Create this structure:
  archivist/
  ├── cmd/
  │ ├── archivist/
  │ │ └── main.go # TUI entry point
  │ └── archivist-server/
  │ └── main.go # Web/API server entry point
  │
  ├── internal/
  │ ├── app/ # Application services
  │ ├── auth/ # Users, sessions, roles, password hashing
  │ ├── documents/ # Uploads, extraction, chunking
  │ ├── embeddings/ # Ollama embedding client
  │ ├── models/ # Ollama chat/model client
  │ ├── rag/ # Retrieval-Augmented Generation
  │ ├── storage/ # SQLite setup and repositories
  │ ├── tui/ # Bubble Tea UI
  │ └── web/
  │ ├── handlers/
  │ ├── middleware/
  │ ├── routes/
  │ ├── templates/
  │ └── viewmodels/
  │
  ├── web/
  │ └── static/
  │ ├── css/
  │ └── js/
  │
  ├── data/
  │ └── .gitkeep
  │
  ├── docs/
  │ ├── architecture.md
  │ ├── setup.md
  │ └── capstone-notes.md
  │
  ├── Dockerfile
  ├── docker-compose.yml
  ├── go.mod
  ├── go.sum
  ├── README.md
  └── .env.example
  Data Model
  Use SQLite.
  Create tables for:
  users
  sessions
  workspaces
  workspace_members
  documents
  document_chunks
  chat_sessions
  chat_messages
  jobs
  audit_logs
  Roles
  Support these roles:
  admin
  student
  MVP permissions:
  Admin can:
- create workspaces
- upload documents
- delete documents
- rebuild workspace index
- create student users
- assign students to workspaces
- chat with any workspace
  Student can:
- view assigned published workspaces
- ask questions in assigned workspaces
- view source citations
  Important Architecture Rule
  Do not put business logic in templates or TUI components.
  Use this pattern:
  HTTP handler / TUI command
  → app service
  → repository / model client / document service
  → view model
  → renderer
  The TUI and GUI should eventually show the same information by consuming shared view models.
  Web Routes
  Implement these MVP routes:
  GET /login
  POST /login
  POST /logout

GET /
GET /dashboard

GET /workspaces
GET /workspaces/new
POST /workspaces
GET /workspaces/{id}

GET /workspaces/{id}/documents
POST /workspaces/{id}/documents
POST /workspaces/{id}/documents/{documentID}/delete
POST /workspaces/{id}/reindex

GET /workspaces/{id}/chat
POST /workspaces/{id}/chat

GET /users
GET /users/new
POST /users
POST /workspaces/{id}/members
Use HTMX for partial updates where it makes sense, especially for:

- document upload results
- workspace panels
- chat messages
- reindex job status
  TUI Requirements
  Create a basic Bubble Tea TUI foundation.
  For now, it only needs to support:

1. App title: Archivist
2. Workspace list screen
3. Workspace detail screen
4. Server status screen
   The TUI does not need full document upload or chat yet. It should compile and establish the structure for future work.
   Document Ingestion MVP
   Support these file types first:
   .txt
   .md
   .pdf
   For .txt and .md, extract plain text directly.
   For .pdf, use a reasonable Go PDF text extraction package. If PDF extraction is difficult, create an interface and a placeholder implementation with clear TODO comments.
   For every uploaded document:
5. Save the original file under /data/uploads/{workspace_id}/.
6. Extract text.
7. Chunk the text.
8. Store chunks in document_chunks.
9. Generate embeddings through Ollama.
10. Store embedding vectors in SQLite as JSON or BLOB for the MVP.
    Use simple chunking:
    chunk size: approximately 800-1200 characters
    overlap: approximately 150-250 characters
    Each chunk should store metadata:
    document_id
    workspace_id
    chunk_index
    content
    source_name
    page_number nullable
    embedding
    created_at
    RAG MVP
    When a student asks a question:
11. Embed the question using Ollama embeddings.
12. Compare the question embedding with stored chunk embeddings.
13. Select the top 3-5 most relevant chunks.
14. Build a prompt that instructs the model to answer only from the provided context.
15. Ask Ollama chat model.
16. Return the response with source references.
    Use cosine similarity for retrieval.
    Prompt style:
    You are Archivist, a course assistant. Answer the student's question using only the provided course context. If the answer is not available in the context, say that the course materials do not provide enough information. Do not invent facts. Include source references after the answer.

Course context:
{{chunks}}

Student question:
{{question}}
Ollama Client
Create a clean Ollama client with methods like:
GenerateEmbedding(ctx context.Context, model string, text string) ([]float64, error)
Chat(ctx context.Context, model string, messages []ChatMessage) (string, error)
ListModels(ctx context.Context) ([]ModelInfo, error)
Health(ctx context.Context) error
Use environment variables:
ARCHIVIST_DB_PATH=/data/archivist.db
ARCHIVIST_STORAGE_PATH=/data/storage
ARCHIVIST_UPLOAD_PATH=/data/uploads
OLLAMA_BASE_URL=http://ollama:11434
ARCHIVIST_DEFAULT_MODEL=gemma3:1b
ARCHIVIST_EMBED_MODEL=nomic-embed-text
ARCHIVIST_HTTP_ADDR=:8080
Docker Requirements
Create a working Docker setup.
docker-compose.yml should include:
archivist
ollama
The app should be available at:
http://localhost:8080
Use named volumes for:
archivist-data
ollama-data
Do not require the user to install Go locally to run the Docker version.
Initial Admin Setup
On first run, if no users exist, show a setup page:
Create Admin Account
After the first admin exists, redirect unauthenticated users to login.
Passwords must be hashed using bcrypt or argon2.
UI Style
Keep the UI simple, clean, and readable.
Use server-rendered HTML with HTMX.
The interface should include:

- login page
- admin dashboard
- workspace list
- workspace detail page
- document upload panel
- simple chat interface
- user management page
  Do not overbuild styling. Use plain CSS or a small custom CSS file.
  README Requirements
  Create a README that explains:

1. What Archivist is
2. Capstone project goal
3. Tech stack
4. How to run with Docker Compose
5. How to pull the default Ollama models
6. How to create the first admin account
7. Current MVP limitations
8. Future roadmap
   Include commands:
   docker compose up --build
   docker compose exec ollama ollama pull gemma3:1b
   docker compose exec ollama ollama pull nomic-embed-text
   Capstone Documentation
   Create docs/capstone-notes.md with:
   Project title:
   Archivist: Investigating Guided Workflows for Reducing Technical Barriers to Local Artificial Intelligence Adoption

Research focus:
This project investigates whether guided workflows can reduce the technical barriers associated with configuring and deploying local AI assistants for educational use.

Primary workflow:
Admin uploads course documents. Students ask questions. Archivist answers using local AI and cites the uploaded course materials.

Primary research question:
Can guided workflows significantly reduce the technical barriers associated with configuring and deploying local AI assistants for educational use?
Implementation Expectations
Build this in small, working steps.
Prioritize:

1. Compiling code
2. Clear architecture
3. Working Docker setup
4. Working login/session flow
5. Working workspace creation
6. Working document upload
7. Working basic RAG
8. Clean documentation
   Do not create incomplete large abstractions. Prefer simple, understandable code.
   First Task
   Start by creating:
9. Go module
10. Project directory structure
11. Dockerfile
12. docker-compose.yml
13. SQLite connection layer
14. Migration setup
15. Basic HTTP server
16. First-run admin setup page
17. Login/logout/session handling
18. Minimal Bubble Tea TUI entry point
    After that, continue with workspace management and document upload.
