#!/bin/bash
echo "Running backend tests..."

echo "Testing GET request to http://localhost:8080/"
curl -i http://localhost:8080/

# --- Test Case 1: PNG Upload ---
echo "Uploading lenna.png..."
curl -i -X POST http://localhost:8080/upload \
  -F "claim_number=CLM-10001" \
  -F "claimant_name=John Doe" \
  -F "date_of_injury=2026-01-15" \
  -F "employer=Acme Corp" \
  -F "adjuster=Jane Smith" \
  -F "support=Full Support" \
  -F "claim_type=Workers Comp" \
  -F "jurisdiction=California" \
  -F "policy_number=POL-998877" \
  -F "acts_id=ACTS_001" \
  -F "data=somedata" \
  -F "file=@lenna.png"

sleep 2

# --- Test Case 2: JPG Upload ---
echo "Uploading lenna.jpg..."
curl -i -X POST http://localhost:8080/upload \
  -F "claim_number=CLM-10002" \
  -F "claimant_name=Alice Smith" \
  -F "date_of_injury=2026-02-20" \
  -F "employer=Stark Industries" \
  -F "adjuster=Robert Downey" \
  -F "support=Minimal" \
  -F "claim_type=Liability" \
  -F "jurisdiction=New York" \
  -F "policy_number=POL-112233" \
  -F "acts_id=ACTS_002" \
  -F "data=somedata" \
  -F "file=@lenna.jpg"

sleep 2

# --- Test Case 3: PDF Upload ---
echo "Uploading test.pdf..."
curl -i -X POST http://localhost:8080/upload \
  -F "claim_number=CLM-10003" \
  -F "claimant_name=Bob Johnson" \
  -F "date_of_injury=2026-03-05" \
  -F "employer=Wayne Enterprises" \
  -F "adjuster=Bruce Wayne" \
  -F "support=None" \
  -F "claim_type=Property" \
  -F "jurisdiction=Gotham" \
  -F "policy_number=POL-445566" \
  -F "acts_id=ACTS_003" \
  -F "data=somedata" \
  -F "file=@test.pdf"

sleep 2

# --- Test Case 4: TXT Upload ---
echo "Uploading bee_moive_script.txt..."
curl -i -X POST http://localhost:8080/upload \
  -F "claim_number=CLM-10004" \
  -F "claimant_name=Barry Benson" \
  -F "date_of_injury=2026-04-12" \
  -F "employer=Honex Industries" \
  -F "adjuster=Vanessa Bloome" \
  -F "support=Legal" \
  -F "claim_type=Personal Injury" \
  -F "jurisdiction=Texas" \
  -F "policy_number=POL-778899" \
  -F "acts_id=ACTS_004" \
  -F "data=somedata" \
  -F "file=@bee_moive_script.txt"

sleep 2

# --- Test Case 5: Custom/No Extension Upload ---
echo "Uploading CrossDocTesting..."
curl -i -X POST http://localhost:8080/upload \
  -F "claim_number=CLM-10005" \
  -F "claimant_name=Charlie Brown" \
  -F "date_of_injury=2026-05-19" \
  -F "employer=Peanuts LLC" \
  -F "adjuster=Linus Van Pelt" \
  -F "support=Psychiatric" \
  -F "claim_type=Medical" \
  -F "jurisdiction=Illinois" \
  -F "policy_number=POL-554433" \
  -F "acts_id=ACTS_005" \
  -F "data=somedata" \
  -F "file=@CrossDocTesting"
