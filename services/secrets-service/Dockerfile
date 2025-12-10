






# Use the official Golang image as the base image
FROM golang:1.25-alpine

# Set the Current Working Directory inside the container
WORKDIR /app

# Copy the go.mod and go.sum files
COPY go.mod go.sum ./

# Download and tidy dependencies to ensure go.sum is up to date
RUN go mod tidy

# Copy the source code into the container
COPY . .

# Build the Go app
RUN go build -o secret-service .

# Expose ports
EXPOSE 50053 8082

# Command to run the executable
CMD ["./secret-service"]












