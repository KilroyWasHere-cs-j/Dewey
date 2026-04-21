# CrossDocTool
This repo is PoC for a document managment backend tool.

App is written fully in golang


graph TD
    A[Ingest file] --> B[Validate file type]
    B --> C[Place file in cache]
    C --> D[Sort file and copy to new location]
    D --> E[Create file record in DB]
    E --> F[Populate associated metadata table]

    G[Handle request] --> H[Determine if metadata needs return]
    H --> I[Search cache for file]
    I --> J[If not found, search DB]
    J --> K[Return file]

