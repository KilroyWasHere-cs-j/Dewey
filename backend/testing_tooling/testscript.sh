#!/bin/bash
echo "Running backend tests..."

echo "Testing GET request to http://localhost:8080/"
curl -i http://localhost:8080/


curl -i -X POST http://localhost:8080/upload \
  -F "ACTS_ID=ACTS_001" \
  -F "data=somedata" \
  -F "file=@lenna.png"

sleep 2

curl -i -X POST http://localhost:8080/upload \
  -F "ACTS_ID=ACTS_002" \
  -F "data=somedata" \
  -F "file=@lenna.jpg"

sleep 2

curl -i -X POST http://localhost:8080/upload \
  -F "ACTS_ID=ACTS_003" \
  -F "data=somedata" \
  -F "file=@test.pdf"

sleep 2

curl -i -X POST http://localhost:8080/upload \
  -F "ACTS_ID=ACTS_004" \
  -F "data=somedata" \
  -F "file=@bee_moive_script.txt"

sleep 2

curl -i -X POST http://localhost:8080/upload \
  -F "ACTS_ID=ACTS_005" \
  -F "data=somedata" \
  -F "file=@CrossDocTesting"
