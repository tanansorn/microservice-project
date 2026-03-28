# Microservice Project (Go + Fiber)

This project is a microservice-based backend built with **Go** using the **Fiber framework**.
It is containerized using **Docker** and orchestrated with **Docker Compose**.

---

# Tech Stack

- Go
- Fiber Framework
- Docker
- Docker Compose
- PostgreSQL

---

# Prerequisites

Make sure you have the following installed:

- Go >= 1.21
- Docker
- Docker Compose

Check installation:

```bash
go version
docker -v
docker compose version
```

---

# Installation

Clone the repository

```bash
git clone <your-repo-url>
cd microservice-project
```

Install Go dependencies

```bash
go mod tidy
```

---

# Environment Setup

Create an `.env` file inside the **deployments** folder.

```
deployments/.env
```

> ⚠️ **Important**
> You must create the `.env` file inside the `deployments` directory before running Docker.
> If there is any problem with service force running it again.

---

# Running the Project (Docker)

Start all services with Docker Compose:

```bash
docker compose -f deployments/docker-compose.yml --env-file deployments/.env up -d --build
```

Stop services:

```bash
docker compose down
```

View logs:

```bash
docker compose logs -f
```

---

# Running the Project (Local Development)

Run the Go application locally:

```bash
go run main.go
```

Or if using a cmd structure:

```bash
go run cmd/main.go
```
