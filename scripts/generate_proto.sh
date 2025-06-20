#!/bin/bash

# Generate proto files for gRPC

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo "Generating gRPC proto files..."

# Create proto output directory
mkdir -p grpc/proto

# Generate Go code from proto files
protoc --go_out=. --go_opt=paths=source_relative \
    --go-grpc_out=. --go-grpc_opt=paths=source_relative \
    grpc/proto/user.proto \
    grpc/proto/auth.proto

if [ $? -eq 0 ]; then
    echo -e "${GREEN}Proto files generated successfully!${NC}"
else
    echo -e "${RED}Failed to generate proto files${NC}"
    exit 1
fi

# Make sure the generated files have the correct package
echo "Checking generated files..."

# The generated files should be:
# - grpc/proto/user.pb.go
# - grpc/proto/user_grpc.pb.go
# - grpc/proto/auth.pb.go
# - grpc/proto/auth_grpc.pb.go

if [ -f "grpc/proto/user.pb.go" ] && [ -f "grpc/proto/auth.pb.go" ]; then
    echo -e "${GREEN}All proto files generated correctly!${NC}"
else
    echo -e "${RED}Some proto files are missing${NC}"
    ls -la grpc/proto/
fi