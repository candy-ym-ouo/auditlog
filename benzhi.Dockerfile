# Keep the complete Go toolchain in the image so the project can be built and
# tested inside the evaluation container on both linux/arm64 and linux/amd64.
FROM golang:1.26

WORKDIR /app

# Download dependencies before copying the rest of the source for cache reuse
# and offline builds after the image has been created.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Verify the baseline source during image construction while retaining all Go
# tooling and source files for later interactive work.
RUN go build ./...

CMD ["bash"]
