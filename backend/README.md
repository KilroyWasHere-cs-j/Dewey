## CrossDocTool

A backend file sorting and storage tool that exposes a RESTful API.


The application is written entirely in Go (Golang), chosen for its balance between simplicity and performance. It offers a clean and efficient development experience while still providing the control, reliability, and speed expected from a compiled language.

Go’s built-in concurrency model—centered around goroutines and channels—makes it particularly well-suited for handling multiple file uploads in parallel without degrading system performance. This allows the application to remain responsive and scalable, even under heavy load.

Additionally, Go’s strong standard library, fast compile times, and straightforward deployment (via static binaries) make it an excellent choice for building and maintaining backend services like this one.


```mermaid
flowchart TD

%% In Path
A[Start Ingest] --> B[Ingest File]
B --> C{Validate File Type}
C -- Invalid --> D[Reject File]
C -- Valid --> E[Place File in Cache]
E --> F[Sort File]
F --> G[Copy File to New Location]
G --> H[Create DB Record]
H --> I[Populate Metadata Table]
I --> J[End Ingest]

%% Out Path
K[Start Request] --> L[Handle Request]
L --> M{Return Metadata Only?}
M -- Yes --> N[Fetch Metadata from DB]
N --> O[Return Metadata]

M -- No --> P[Search Cache]
P --> Q{File Found in Cache?}
Q -- Yes --> R[Return File from Cache]
Q -- No --> S[Search Database]
S --> T{File Found in DB?}
T -- Yes --> U[Return File from DB]
T -- No --> V[Return Not Found]

R --> W[End Request]
U --> W
O --> W
V --> W
```
