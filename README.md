# CrossDocTool
This repo is PoC for a document managment backend tool.

App is written fully in golang

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
