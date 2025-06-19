job "boilerplate-api" {
  datacenters = ["dc1"]
  type = "service"
  
  # Update strategy
  update {
    max_parallel      = 2
    min_healthy_time  = "10s"
    healthy_deadline  = "3m"
    progress_deadline = "10m"
    auto_revert       = true
    canary            = 1
  }

  # API Service Group
  group "api" {
    count = 3

    # Scaling policy
    scaling {
      enabled = true
      min     = 2
      max     = 10

      policy {
        # CPU-based autoscaling
        check "cpu" {
          source = "prometheus"
          query  = "avg(nomad_client_allocs_cpu_allocated_percentage{task='api'})"
          
          strategy "target-value" {
            target = 70
          }
        }

        # Memory-based autoscaling
        check "mem" {
          source = "prometheus"
          query  = "avg(nomad_client_allocs_memory_allocated_percentage{task='api'})"
          
          strategy "target-value" {
            target = 80
          }
        }
      }
    }

    restart {
      attempts = 3
      interval = "30m"
      delay    = "15s"
      mode     = "fail"
    }

    ephemeral_disk {
      size = 300
    }

    network {
      port "http" {
        to = 8080
      }
    }

    service {
      name = "boilerplate-api"
      tags = ["api", "http"]
      port = "http"

      check {
        name     = "api_health"
        type     = "http"
        path     = "/health"
        interval = "10s"
        timeout  = "2s"
      }

      connect {
        sidecar_service {}
      }
    }

    task "api" {
      driver = "docker"

      config {
        image = "boilerplate-api:latest"
        ports = ["http"]
        
        volumes = [
          "local/uploads:/app/uploads",
          "local/videos:/app/videos",
          "secrets/env:/app/.env"
        ]

        logging {
          type = "json-file"
          config {
            max-files = 10
            max-size = "10m"
          }
        }
      }

      env {
        APP_ENV  = "production"
        APP_PORT = "${NOMAD_PORT_http}"
      }

      template {
        data = <<EOH
DB_HOST={{ with service "postgres" }}{{ with index . 0 }}{{ .Address }}{{ end }}{{ end }}
DB_PORT={{ with service "postgres" }}{{ with index . 0 }}{{ .Port }}{{ end }}{{ end }}
DB_USER={{with secret "secret/data/boilerplate/db"}}{{.Data.data.username}}{{end}}
DB_PASSWORD={{with secret "secret/data/boilerplate/db"}}{{.Data.data.password}}{{end}}
DB_NAME=boilerplate

REDIS_HOST={{ with service "redis" }}{{ with index . 0 }}{{ .Address }}{{ end }}{{ end }}
REDIS_PORT={{ with service "redis" }}{{ with index . 0 }}{{ .Port }}{{ end }}{{ end }}

JWT_SECRET={{with secret "secret/data/boilerplate/jwt"}}{{.Data.data.secret}}{{end}}
ENCRYPTION_KEY={{with secret "secret/data/boilerplate/encryption"}}{{.Data.data.key}}{{end}}
EOH
        destination = "secrets/env"
        env         = true
      }

      resources {
        cpu    = 500
        memory = 512
      }

      vault {
        policies = ["boilerplate-api"]
      }
    }
  }

  # gRPC Service Group
  group "grpc" {
    count = 2

    scaling {
      enabled = true
      min     = 1
      max     = 5

      policy {
        check "cpu" {
          source = "prometheus"
          query  = "avg(nomad_client_allocs_cpu_allocated_percentage{task='grpc'})"
          
          strategy "target-value" {
            target = 70
          }
        }
      }
    }

    restart {
      attempts = 3
      interval = "30m"
      delay    = "15s"
      mode     = "fail"
    }

    network {
      port "grpc" {
        to = 50051
      }
    }

    service {
      name = "boilerplate-grpc"
      tags = ["grpc"]
      port = "grpc"

      check {
        name     = "grpc_health"
        type     = "grpc"
        interval = "10s"
        timeout  = "2s"
      }

      connect {
        sidecar_service {}
      }
    }

    task "grpc" {
      driver = "docker"

      config {
        image = "boilerplate-api:latest"
        command = "/app/grpc"
        ports = ["grpc"]
        
        volumes = [
          "secrets/env:/app/.env"
        ]
      }

      env {
        APP_ENV   = "production"
        GRPC_PORT = "${NOMAD_PORT_grpc}"
      }

      template {
        data = <<EOH
DB_HOST={{ with service "postgres" }}{{ with index . 0 }}{{ .Address }}{{ end }}{{ end }}
DB_PORT={{ with service "postgres" }}{{ with index . 0 }}{{ .Port }}{{ end }}{{ end }}
DB_USER={{with secret "secret/data/boilerplate/db"}}{{.Data.data.username}}{{end}}
DB_PASSWORD={{with secret "secret/data/boilerplate/db"}}{{.Data.data.password}}{{end}}
DB_NAME=boilerplate

REDIS_HOST={{ with service "redis" }}{{ with index . 0 }}{{ .Address }}{{ end }}{{ end }}
REDIS_PORT={{ with service "redis" }}{{ with index . 0 }}{{ .Port }}{{ end }}{{ end }}

JWT_SECRET={{with secret "secret/data/boilerplate/jwt"}}{{.Data.data.secret}}{{end}}
EOH
        destination = "secrets/env"
        env         = true
      }

      resources {
        cpu    = 250
        memory = 256
      }

      vault {
        policies = ["boilerplate-api"]
      }
    }
  }

  # Background Jobs Group
  group "jobs" {
    count = 1

    restart {
      attempts = 3
      interval = "30m"
      delay    = "15s"
      mode     = "fail"
    }

    task "worker" {
      driver = "docker"

      config {
        image = "boilerplate-api:latest"
        command = "/app/worker"
        
        volumes = [
          "local/uploads:/app/uploads",
          "local/videos:/app/videos",
          "secrets/env:/app/.env"
        ]
      }

      env {
        APP_ENV = "production"
      }

      template {
        data = <<EOH
DB_HOST={{ with service "postgres" }}{{ with index . 0 }}{{ .Address }}{{ end }}{{ end }}
DB_PORT={{ with service "postgres" }}{{ with index . 0 }}{{ .Port }}{{ end }}{{ end }}
DB_USER={{with secret "secret/data/boilerplate/db"}}{{.Data.data.username}}{{end}}
DB_PASSWORD={{with secret "secret/data/boilerplate/db"}}{{.Data.data.password}}{{end}}
DB_NAME=boilerplate

REDIS_HOST={{ with service "redis" }}{{ with index . 0 }}{{ .Address }}{{ end }}{{ end }}
REDIS_PORT={{ with service "redis" }}{{ with index . 0 }}{{ .Port }}{{ end }}{{ end }}
EOH
        destination = "secrets/env"
        env         = true
      }

      resources {
        cpu    = 200
        memory = 256
      }

      vault {
        policies = ["boilerplate-api"]
      }
    }
  }
}