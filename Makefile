.PHONY: test backend frontend

test:
	cd backend && go test ./...

backend:
	cd backend && go run ./cmd/server

frontend:
	cd frontend && npm install && npm run dev
