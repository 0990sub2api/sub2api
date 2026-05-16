#!/bin/bash
# =============================================================================
# Sub2API SOCKS5 Docker Deployment Preparation Script
# =============================================================================
# This script prepares deployment files for Sub2API:
#   - Downloads docker-compose.socks5.yml and .env.example
#   - Generates secure secrets (JWT_SECRET, TOTP_ENCRYPTION_KEY, POSTGRES_PASSWORD, ADMIN_PASSWORD)
#   - Reads SUB2API_IMAGE, SERVER_PORT, and FORCED_OPENAI_OAUTH_SOCKS5_* from docker-socks5-deploy.env
#   - Creates necessary data directories
#
# After running this script, you can start services with:
#   docker-compose up -d
# =============================================================================

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# GitHub raw content base URL
GITHUB_RAW_URL="${GITHUB_RAW_URL:-https://raw.githubusercontent.com/0990sub2api/sub2api/main/deploy}"
DEPLOY_CONFIG_FILE="docker-socks5-deploy.env"

# Print colored message
print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Generate random secret
generate_secret() {
    openssl rand -hex 32
}

# Check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

escape_sed_replacement() {
    printf '%s' "$1" | sed 's/[\/&]/\\&/g'
}

# Main installation function
main() {
    echo ""
    echo "=========================================="
    echo "  Sub2API SOCKS5 Deployment Preparation"
    echo "=========================================="
    echo ""

    if [ ! -f "${DEPLOY_CONFIG_FILE}" ]; then
        print_warning "${DEPLOY_CONFIG_FILE} not found in current directory."
        print_info "Downloading ${DEPLOY_CONFIG_FILE}..."
        if command_exists curl; then
            curl -sSL "${GITHUB_RAW_URL}/${DEPLOY_CONFIG_FILE}" -o "${DEPLOY_CONFIG_FILE}"
        elif command_exists wget; then
            wget -q "${GITHUB_RAW_URL}/${DEPLOY_CONFIG_FILE}" -O "${DEPLOY_CONFIG_FILE}"
        else
            print_error "Neither curl nor wget is installed. Please install one of them."
            exit 1
        fi
        print_success "Downloaded ${DEPLOY_CONFIG_FILE}"
        print_warning "Please edit ${DEPLOY_CONFIG_FILE}, then run this script again to install."
        exit 0
    fi

    # shellcheck disable=SC1090
    set -a
    . "./${DEPLOY_CONFIG_FILE}"
    set +a

    : "${SUB2API_IMAGE:?SUB2API_IMAGE is required in ${DEPLOY_CONFIG_FILE}}"
    : "${SERVER_PORT:?SERVER_PORT is required in ${DEPLOY_CONFIG_FILE}}"
    : "${FORCED_OPENAI_OAUTH_SOCKS5_HOST:?FORCED_OPENAI_OAUTH_SOCKS5_HOST is required in ${DEPLOY_CONFIG_FILE}}"
    : "${FORCED_OPENAI_OAUTH_SOCKS5_PORT:?FORCED_OPENAI_OAUTH_SOCKS5_PORT is required in ${DEPLOY_CONFIG_FILE}}"
    : "${FORCED_OPENAI_OAUTH_SOCKS5_PASSWORD:?FORCED_OPENAI_OAUTH_SOCKS5_PASSWORD is required in ${DEPLOY_CONFIG_FILE}}"

    # Check if openssl is available
    if ! command_exists openssl; then
        print_error "openssl is not installed. Please install openssl first."
        exit 1
    fi

    # Check if deployment already exists
    if [ -f "docker-compose.yml" ] && [ -f ".env" ]; then
        print_warning "Deployment files already exist in current directory."
        read -p "Overwrite existing files? (y/N): " -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            print_info "Cancelled."
            exit 0
        fi
    fi

    # Download docker-compose.socks5.yml and save as docker-compose.yml
    print_info "Downloading docker-compose.yml..."
    if command_exists curl; then
        curl -sSL "${GITHUB_RAW_URL}/docker-compose.socks5.yml" -o docker-compose.yml
    elif command_exists wget; then
        wget -q "${GITHUB_RAW_URL}/docker-compose.socks5.yml" -O docker-compose.yml
    else
        print_error "Neither curl nor wget is installed. Please install one of them."
        exit 1
    fi
    print_success "Downloaded docker-compose.yml"

    # Download .env.example
    print_info "Downloading .env.example..."
    if command_exists curl; then
        curl -sSL "${GITHUB_RAW_URL}/.env.example" -o .env.example
    else
        wget -q "${GITHUB_RAW_URL}/.env.example" -O .env.example
    fi
    print_success "Downloaded .env.example"

    # Generate .env file with auto-generated secrets
    print_info "Generating secure secrets..."
    echo ""

    # Generate secrets
    JWT_SECRET=$(generate_secret)
    TOTP_ENCRYPTION_KEY=$(generate_secret)
    POSTGRES_PASSWORD=$(generate_secret)
    ADMIN_PASSWORD=$(generate_secret)
    SUB2API_IMAGE_ESCAPED=$(escape_sed_replacement "${SUB2API_IMAGE}")
    SERVER_PORT_ESCAPED=$(escape_sed_replacement "${SERVER_PORT}")
    FORCED_OPENAI_OAUTH_SOCKS5_HOST_ESCAPED=$(escape_sed_replacement "${FORCED_OPENAI_OAUTH_SOCKS5_HOST}")
    FORCED_OPENAI_OAUTH_SOCKS5_PORT_ESCAPED=$(escape_sed_replacement "${FORCED_OPENAI_OAUTH_SOCKS5_PORT}")
    FORCED_OPENAI_OAUTH_SOCKS5_PASSWORD_ESCAPED=$(escape_sed_replacement "${FORCED_OPENAI_OAUTH_SOCKS5_PASSWORD}")

    # Create .env from .env.example
    cp .env.example .env

    # Update .env with generated secrets (cross-platform compatible)
    if sed --version >/dev/null 2>&1; then
        # GNU sed (Linux)
        sed -i "s/^JWT_SECRET=.*/JWT_SECRET=${JWT_SECRET}/" .env
        sed -i "s/^TOTP_ENCRYPTION_KEY=.*/TOTP_ENCRYPTION_KEY=${TOTP_ENCRYPTION_KEY}/" .env
        sed -i "s/^POSTGRES_PASSWORD=.*/POSTGRES_PASSWORD=${POSTGRES_PASSWORD}/" .env
        sed -i "s/^ADMIN_PASSWORD=.*/ADMIN_PASSWORD=${ADMIN_PASSWORD}/" .env
        sed -i "s/^SUB2API_IMAGE=.*/SUB2API_IMAGE=${SUB2API_IMAGE_ESCAPED}/" .env
        sed -i "s/^SERVER_PORT=.*/SERVER_PORT=${SERVER_PORT_ESCAPED}/" .env
        sed -i "s/^FORCED_OPENAI_OAUTH_SOCKS5_HOST=.*/FORCED_OPENAI_OAUTH_SOCKS5_HOST=${FORCED_OPENAI_OAUTH_SOCKS5_HOST_ESCAPED}/" .env
        sed -i "s/^FORCED_OPENAI_OAUTH_SOCKS5_PORT=.*/FORCED_OPENAI_OAUTH_SOCKS5_PORT=${FORCED_OPENAI_OAUTH_SOCKS5_PORT_ESCAPED}/" .env
        sed -i "s/^FORCED_OPENAI_OAUTH_SOCKS5_PASSWORD=.*/FORCED_OPENAI_OAUTH_SOCKS5_PASSWORD=${FORCED_OPENAI_OAUTH_SOCKS5_PASSWORD_ESCAPED}/" .env
    else
        # BSD sed (macOS)
        sed -i '' "s/^JWT_SECRET=.*/JWT_SECRET=${JWT_SECRET}/" .env
        sed -i '' "s/^TOTP_ENCRYPTION_KEY=.*/TOTP_ENCRYPTION_KEY=${TOTP_ENCRYPTION_KEY}/" .env
        sed -i '' "s/^POSTGRES_PASSWORD=.*/POSTGRES_PASSWORD=${POSTGRES_PASSWORD}/" .env
        sed -i '' "s/^ADMIN_PASSWORD=.*/ADMIN_PASSWORD=${ADMIN_PASSWORD}/" .env
        sed -i '' "s/^SUB2API_IMAGE=.*/SUB2API_IMAGE=${SUB2API_IMAGE_ESCAPED}/" .env
        sed -i '' "s/^SERVER_PORT=.*/SERVER_PORT=${SERVER_PORT_ESCAPED}/" .env
        sed -i '' "s/^FORCED_OPENAI_OAUTH_SOCKS5_HOST=.*/FORCED_OPENAI_OAUTH_SOCKS5_HOST=${FORCED_OPENAI_OAUTH_SOCKS5_HOST_ESCAPED}/" .env
        sed -i '' "s/^FORCED_OPENAI_OAUTH_SOCKS5_PORT=.*/FORCED_OPENAI_OAUTH_SOCKS5_PORT=${FORCED_OPENAI_OAUTH_SOCKS5_PORT_ESCAPED}/" .env
        sed -i '' "s/^FORCED_OPENAI_OAUTH_SOCKS5_PASSWORD=.*/FORCED_OPENAI_OAUTH_SOCKS5_PASSWORD=${FORCED_OPENAI_OAUTH_SOCKS5_PASSWORD_ESCAPED}/" .env
    fi

    if ! grep -q '^SUB2API_IMAGE=' .env; then
        {
            echo ""
            echo "# Docker Image"
            echo "SUB2API_IMAGE=${SUB2API_IMAGE}"
        } >> .env
    fi

    if ! grep -q '^FORCED_OPENAI_OAUTH_SOCKS5_HOST=' .env; then
        {
            echo ""
            echo "# Forced OpenAI OAuth SOCKS5 Proxy"
            echo "FORCED_OPENAI_OAUTH_SOCKS5_HOST=${FORCED_OPENAI_OAUTH_SOCKS5_HOST}"
            echo "FORCED_OPENAI_OAUTH_SOCKS5_PORT=${FORCED_OPENAI_OAUTH_SOCKS5_PORT}"
            echo "FORCED_OPENAI_OAUTH_SOCKS5_PASSWORD=${FORCED_OPENAI_OAUTH_SOCKS5_PASSWORD}"
        } >> .env
    fi

    # Create data directories
    print_info "Creating data directories..."
    mkdir -p data postgres_data redis_data
    print_success "Created data directories"

    # Set secure permissions for .env file (readable/writable only by owner)
    chmod 600 .env
    echo ""

    # Display completion message
    echo "=========================================="
    echo "  Preparation Complete!"
    echo "=========================================="
    echo ""
    echo "Generated secure credentials:"
    echo "  POSTGRES_PASSWORD:     ${POSTGRES_PASSWORD}"
    echo "  ADMIN_PASSWORD:        ${ADMIN_PASSWORD}"
    echo "  JWT_SECRET:            ${JWT_SECRET}"
    echo "  TOTP_ENCRYPTION_KEY:   ${TOTP_ENCRYPTION_KEY}"
    echo "  SUB2API_IMAGE:         ${SUB2API_IMAGE}"
    echo "  SERVER_PORT:           ${SERVER_PORT}"
    echo "  SOCKS5_HOST:           ${FORCED_OPENAI_OAUTH_SOCKS5_HOST}"
    echo "  SOCKS5_PORT:           ${FORCED_OPENAI_OAUTH_SOCKS5_PORT}"
    echo ""
    print_warning "These credentials have been saved to .env file."
    print_warning "Please keep them secure and do not share publicly!"
    echo ""
    echo "Directory structure:"
    echo "  docker-compose.yml        - Docker Compose configuration"
    echo "  .env                      - Environment variables (generated secrets)"
    echo "  .env.example              - Example template (for reference)"
    echo "  ${DEPLOY_CONFIG_FILE}     - Deployment input configuration"
    echo "  data/                     - Application data (will be created on first run)"
    echo "  postgres_data/            - PostgreSQL data"
    echo "  redis_data/               - Redis data"
    echo ""
    echo "Next steps:"
    echo "  1. (Optional) Edit .env to customize configuration"
    echo "  2. Start services:"
    echo "     docker-compose up -d"
    echo ""
    echo "  3. View logs:"
    echo "     docker-compose logs -f sub2api"
    echo ""
    echo "  4. Access Web UI:"
    echo "     http://localhost:${SERVER_PORT}"
    echo ""
    print_info "Admin password has been generated and saved to .env."
    echo ""
}

# Run main function
main "$@"
