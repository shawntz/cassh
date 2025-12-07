#!/bin/bash
# deploy-gcp.sh - Deploy cassh to Google Cloud Run
#
# Prerequisites:
#   1. Install Google Cloud CLI: https://cloud.google.com/sdk/docs/install
#   2. Authenticate: gcloud auth login
#   3. Set project: gcloud config set project YOUR_PROJECT_ID
#   4. Enable APIs: gcloud services enable run.googleapis.com containerregistry.googleapis.com
#
# Usage:
#   ./scripts/deploy-gcp.sh
#
# Required environment variables (set these before running):
#   CASSH_OIDC_CLIENT_ID      - Microsoft Entra app client ID
#   CASSH_OIDC_CLIENT_SECRET  - Microsoft Entra app client secret
#   CASSH_OIDC_TENANT         - Microsoft Entra tenant ID
#   CASSH_CA_PRIVATE_KEY      - CA private key (full PEM content)
#
# Optional environment variables:
#   GCP_PROJECT               - GCP project ID (defaults to current project)
#   GCP_REGION                - GCP region (defaults to us-central1)
#   SERVICE_NAME              - Cloud Run service name (defaults to cassh)

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}🚀 Deploying cassh to Google Cloud Run${NC}"
echo ""

# Check for gcloud CLI
if ! command -v gcloud &> /dev/null; then
    echo -e "${RED}Error: gcloud CLI not found${NC}"
    echo "Install it from: https://cloud.google.com/sdk/docs/install"
    exit 1
fi

# Get project ID
GCP_PROJECT=${GCP_PROJECT:-$(gcloud config get-value project 2>/dev/null)}
if [ -z "$GCP_PROJECT" ]; then
    echo -e "${RED}Error: No GCP project configured${NC}"
    echo "Run: gcloud config set project YOUR_PROJECT_ID"
    exit 1
fi

# Configuration
GCP_REGION=${GCP_REGION:-us-central1}
SERVICE_NAME=${SERVICE_NAME:-cassh}
IMAGE_NAME="gcr.io/${GCP_PROJECT}/${SERVICE_NAME}"

echo -e "${YELLOW}Configuration:${NC}"
echo "  Project:  $GCP_PROJECT"
echo "  Region:   $GCP_REGION"
echo "  Service:  $SERVICE_NAME"
echo "  Image:    $IMAGE_NAME"
echo ""

# Validate required environment variables
missing_vars=()
[ -z "$CASSH_OIDC_CLIENT_ID" ] && missing_vars+=("CASSH_OIDC_CLIENT_ID")
[ -z "$CASSH_OIDC_CLIENT_SECRET" ] && missing_vars+=("CASSH_OIDC_CLIENT_SECRET")
[ -z "$CASSH_OIDC_TENANT" ] && missing_vars+=("CASSH_OIDC_TENANT")
[ -z "$CASSH_CA_PRIVATE_KEY" ] && missing_vars+=("CASSH_CA_PRIVATE_KEY")

if [ ${#missing_vars[@]} -ne 0 ]; then
    echo -e "${RED}Error: Missing required environment variables:${NC}"
    for var in "${missing_vars[@]}"; do
        echo "  - $var"
    done
    echo ""
    echo "Set these variables before running the script:"
    echo "  export CASSH_OIDC_CLIENT_ID='your-client-id'"
    echo "  export CASSH_OIDC_CLIENT_SECRET='your-client-secret'"
    echo "  export CASSH_OIDC_TENANT='your-tenant-id'"
    echo "  export CASSH_CA_PRIVATE_KEY='\$(cat ca_key)'"
    exit 1
fi

# Enable required APIs
echo -e "${YELLOW}Enabling required APIs...${NC}"
gcloud services enable run.googleapis.com containerregistry.googleapis.com --quiet

# Build and push image
echo -e "${YELLOW}Building and pushing Docker image...${NC}"
cd "$(dirname "$0")/.."

# Configure Docker for GCR
gcloud auth configure-docker --quiet

# Build image
docker build -t "$IMAGE_NAME" .

# Push image
docker push "$IMAGE_NAME"

# Deploy to Cloud Run
echo -e "${YELLOW}Deploying to Cloud Run...${NC}"

# First deploy to get the service URL
gcloud run deploy "$SERVICE_NAME" \
    --image "$IMAGE_NAME" \
    --platform managed \
    --region "$GCP_REGION" \
    --allow-unauthenticated \
    --port 8080 \
    --memory 256Mi \
    --cpu 1 \
    --min-instances 0 \
    --max-instances 10 \
    --set-env-vars "CASSH_OIDC_CLIENT_ID=$CASSH_OIDC_CLIENT_ID" \
    --set-env-vars "CASSH_OIDC_CLIENT_SECRET=$CASSH_OIDC_CLIENT_SECRET" \
    --set-env-vars "CASSH_OIDC_TENANT=$CASSH_OIDC_TENANT" \
    --set-env-vars "CASSH_CA_PRIVATE_KEY=$CASSH_CA_PRIVATE_KEY" \
    --set-env-vars "CASSH_CERT_VALIDITY_HOURS=${CASSH_CERT_VALIDITY_HOURS:-12}" \
    --set-env-vars "CASSH_GITHUB_PRINCIPAL_SOURCE=${CASSH_GITHUB_PRINCIPAL_SOURCE:-email_prefix}" \
    --quiet

# Get the service URL
SERVICE_URL=$(gcloud run services describe "$SERVICE_NAME" --region "$GCP_REGION" --format 'value(status.url)')

# Update with the correct server URL for OAuth redirects
echo -e "${YELLOW}Updating service with OAuth redirect URL...${NC}"
gcloud run services update "$SERVICE_NAME" \
    --region "$GCP_REGION" \
    --set-env-vars "CASSH_SERVER_URL=$SERVICE_URL" \
    --set-env-vars "CASSH_OIDC_REDIRECT_URL=${SERVICE_URL}/auth/callback" \
    --quiet

echo ""
echo -e "${GREEN}✅ Deployment complete!${NC}"
echo ""
echo -e "${YELLOW}Service URL:${NC} $SERVICE_URL"
echo ""
echo -e "${YELLOW}Next steps:${NC}"
echo "1. Add the OAuth redirect URL to your Microsoft Entra app:"
echo "   ${SERVICE_URL}/auth/callback"
echo ""
echo "2. Configure your cassh client with this server URL:"
echo "   Server URL: $SERVICE_URL"
echo ""
echo "3. Test the deployment:"
echo "   curl $SERVICE_URL/health"
echo ""
echo -e "${YELLOW}Useful commands:${NC}"
echo "  View logs:     gcloud run logs read --service $SERVICE_NAME --region $GCP_REGION"
echo "  View service:  gcloud run services describe $SERVICE_NAME --region $GCP_REGION"
echo "  Delete:        gcloud run services delete $SERVICE_NAME --region $GCP_REGION"
