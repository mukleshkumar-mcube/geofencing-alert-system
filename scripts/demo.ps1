# Walk the sample fleet against a running API on http://localhost:8080.
$base = "http://localhost:8080"
$ts = [DateTime]::UtcNow.ToString("yyyy-MM-ddTHH:mm:ssZ")

Write-Host "Health"
Invoke-RestMethod "$base/health" | ConvertTo-Json

Write-Host "Move John Doe into Downtown Delivery Zone"
$body = @{
  vehicle_id = "veh_john"
  latitude = 37.7799
  longitude = -122.4144
  timestamp = $ts
} | ConvertTo-Json
Invoke-RestMethod -Method Post -Uri "$base/vehicles/location" -ContentType "application/json" -Body $body | ConvertTo-Json -Depth 6

Write-Host "Recent crossings"
Invoke-RestMethod "$base/violations/history?vehicle_id=veh_john&limit=5" | ConvertTo-Json -Depth 6
