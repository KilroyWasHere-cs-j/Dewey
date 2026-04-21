# 🧪 API Test Commands (curl)

Below is a list of curl commands for testing the servers functionality

Assuming your server is running at:

```
http://localhost:8080
```

---

## 🔹 Basic GET routes

### `/`
```bash
curl -i http://localhost:8080/
```

### `/admin`
```bash
curl -i http://localhost:8080/admin
```

### `/settings`
```bash
curl -i http://localhost:8080/settings
```

---

## 🔹 Upload (POST `/upload`)

### ✅ JSON request
```bash
curl -i -X POST http://localhost:8080/upload \
  -H "Content-Type: application/json" \
  -d '{
    "name": "test",
    "data": "example"
  }'
```

---

### ✅ Multipart (file + fields)
```bash
curl -i -X POST http://localhost:8080/upload \
  -F "name=myfile" \
  -F "data=somedata" \
  -F "file=@/path/to/your/file.txt"
```

---

### ❌ Invalid content-type (error case)
```bash
curl -i -X POST http://localhost:8080/upload \
  -H "Content-Type: text/plain" \
  -d "invalid"
```

---

## 🔹 Get a specific file

### `/files/:filename/:meta`
```bash
curl -i http://localhost:8080/files/example.txt/someMeta
```

---

## 🔹 List files

### `/files`
```bash
curl -i http://localhost:8080/files
```

---

## 🔹 Delete a file

### `/files/:filename`
```bash
curl -i -X DELETE http://localhost:8080/files/example.txt
```

---

## 🔹 Trigger cache dump

### `/admin/dumpCache`
```bash
curl -i http://localhost:8080/admin/dumpCache
```

---

## 🧪 Tip: verbose output
```bash
curl -v http://localhost:8080/files
```
