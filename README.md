
# Shockbliss - E-commerce Backend API
Welcome to the Shockbliss backend API! This project powers a single-page application, integrating with Paytrail for payments and leveraging Kong Konnekt as an API Gateway. Built with Go.

Technologies Used

Go (Golang): The primary programming language.

Gorilla Mux: Powerful HTTP router for building robust API endpoints.

jmoiron/sqlx: Extensions to database/sql for simpler interaction with PostgreSQL.

lib/pq: PostgreSQL driver for Go.

golang-jwt/jwt: For JSON Web Token (JWT) handling.

go-resty/resty/v2: HTTP client for making API calls (to Paytrail).

joho/godotenv: For loading environment variables from .env files during development.

go-playground/validator: For request payload validation.

go.uber.org/zap: High-performance, structured logging.

rs/cors: Cross-Origin Resource Sharing (CORS) middleware.

golang.org/x/time: For rate limiting.

google/uuid: For generating universally unique identifiers.

golang-migrate/migrate/v4: For database migrations.

gopkg.in/mail.v2: For sending emails (order confirmations).

json-iterator/go: Faster JSON marshaling/unmarshaling.

heptiolabs/healthcheck: For robust health check endpoints
