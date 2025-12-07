#!/bin/bash
# deploy-azure.sh - Deploy cassh to Azure Container Apps
#
# Prerequisites:
#   1. Install Azure CLI: https://docs.microsoft.com/en-us/cli/azure/install-azure-cli
#   2. Login: az login
#   3. Install Docker: https://docs.docker.com/get-docker/
#
# Usage:
#   ./scripts/deploy-azure.sh
#
# Required environment variables (set these before running):
#   CASSH_OIDC_CLIENT_ID      - Microsoft Entra app client ID
#   CASSH_OIDC_CLIENT_SECRET  - Microsoft Entra app client secret
#   CASSH_OIDC_TENANT         - Microsoft Entra tenant ID
#   CASSH_CA_PRIVATE_KEY      - CA private key (full PEM content)
#
# Optional environment variables:
#   AZURE_RESOURCE_GROUP      - Resource group name (defaults to cassh-rg)
#   AZURE_LOCATION            - Azure region (defaults to eastus)
#   SERVICE_NAME              - Container app name (defaults to cassh)

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}🚀 Deploying cassh to Azure Container Apps${NC}"
echo ""

# Check for Azure CLI
if ! command -v az &> /dev/null; then
    echo -e "${RED}Error: Azure CLI not found${NC}"
    echo "Install it from: https://docs.microsoft.com/en-us/cli/azure/install-azure-cli"
    exit 1
fi

# Check for Docker
if ! command -v docker &> /dev/null; then
    echo -e "${RED}Error: Docker not found${NC}"
    echo "Install it from: https://docs.docker.com/get-docker/"
    exit 1
fi

# Check if logged in
if ! az account show &> /dev/null; then
    echo -e "${RED}Error: Not logged into Azure${NC}"
    echo "Run: az login"
    exit 1
fi

# Configuration
AZURE_RESOURCE_GROUP=${AZURE_RESOURCE_GROUP:-cassh-rg}
AZURE_LOCATION=${AZURE_LOCATION:-eastus}
SERVICE_NAME=${SERVICE_NAME:-cassh}
ACR_NAME="${SERVICE_NAME}acr$(date +%s | tail -c 6)"  # Unique ACR name
ENVIRONMENT_NAME="${SERVICE_NAME}-env"

echo -e "${YELLOW}Configuration:${NC}"
echo "  Resource Group: $AZURE_RESOURCE_GROUP"
echo "  Location:       $AZURE_LOCATION"
echo "  Service:        $SERVICE_NAME"
echo "  ACR:            $ACR_NAME"
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

# Create resource group if it doesn't exist
echo -e "${YELLOW}Creating resource group...${NC}"
az group create --name "$AZURE_RESOURCE_GROUP" --location "$AZURE_LOCATION" --output none

# Install/upgrade containerapp extension
echo -e "${YELLOW}Ensuring Container Apps extension is installed...${NC}"
az extension add --name containerapp --upgrade --yes 2>/dev/null || true

# Register required providers
echo -e "${YELLOW}Registering required providers...${NC}"
az provider register --namespace Microsoft.App --wait
az provider register --namespace Microsoft.OperationalInsights --wait

# Create Azure Container Registry
echo -e "${YELLOW}Creating Azure Container Registry...${NC}"
# Check if we already have an ACR in this resource group
EXISTING_ACR=$(az acr list --resource-group "$AZURE_RESOURCE_GROUP" --query '[0].name' --output tsv 2>/dev/null || echo "")
if [ -n "$EXISTING_ACR" ]; then
    ACR_NAME="$EXISTING_ACR"
    echo "Using existing ACR: $ACR_NAME"
else
    az acr create --resource-group "$AZURE_RESOURCE_GROUP" --name "$ACR_NAME" --sku Basic --admin-enabled true --output none
fi

ACR_SERVER=$(az acr show --name "$ACR_NAME" --query 'loginServer' --output tsv)
ACR_PASSWORD=$(az acr credential show --name "$ACR_NAME" --query 'passwords[0].value' --output tsv)

# Build and push image
echo -e "${YELLOW}Building and pushing Docker image...${NC}"
cd "$(dirname "$0")/.."

# Login to ACR
az acr login --name "$ACR_NAME"

# Build and push
docker build -t "${ACR_SERVER}/${SERVICE_NAME}:latest" .
docker push "${ACR_SERVER}/${SERVICE_NAME}:latest"

# Create Container Apps environment if it doesn't exist
echo -e "${YELLOW}Creating Container Apps environment...${NC}"
if ! az containerapp env show --name "$ENVIRONMENT_NAME" --resource-group "$AZURE_RESOURCE_GROUP" &> /dev/null; then
    az containerapp env create \
        --name "$ENVIRONMENT_NAME" \
        --resource-group "$AZURE_RESOURCE_GROUP" \
        --location "$AZURE_LOCATION" \
        --output none
fi

# Deploy Container App
echo -e "${YELLOW}Deploying Container App...${NC}"

# Check if app already exists
if az containerapp show --name "$SERVICE_NAME" --resource-group "$AZURE_RESOURCE_GROUP" &> /dev/null; then
    # Update existing app
    echo "Updating existing Container App..."
    az containerapp update \
        --name "$SERVICE_NAME" \
        --resource-group "$AZURE_RESOURCE_GROUP" \
        --image "${ACR_SERVER}/${SERVICE_NAME}:latest" \
        --set-env-vars \
            "CASSH_OIDC_CLIENT_ID=$CASSH_OIDC_CLIENT_ID" \
            "CASSH_OIDC_CLIENT_SECRET=$CASSH_OIDC_CLIENT_SECRET" \
            "CASSH_OIDC_TENANT=$CASSH_OIDC_TENANT" \
            "CASSH_CA_PRIVATE_KEY=$CASSH_CA_PRIVATE_KEY" \
            "CASSH_CERT_VALIDITY_HOURS=${CASSH_CERT_VALIDITY_HOURS:-12}" \
            "CASSH_GITHUB_PRINCIPAL_SOURCE=${CASSH_GITHUB_PRINCIPAL_SOURCE:-email_prefix}" \
        --output none
else
    # Create new app
    echo "Creating new Container App..."
    az containerapp create \
        --name "$SERVICE_NAME" \
        --resource-group "$AZURE_RESOURCE_GROUP" \
        --environment "$ENVIRONMENT_NAME" \
        --image "${ACR_SERVER}/${SERVICE_NAME}:latest" \
        --registry-server "$ACR_SERVER" \
        --registry-username "$ACR_NAME" \
        --registry-password "$ACR_PASSWORD" \
        --target-port 8080 \
        --ingress external \
        --cpu 0.25 \
        --memory 0.5Gi \
        --min-replicas 0 \
        --max-replicas 10 \
        --env-vars \
            "CASSH_OIDC_CLIENT_ID=$CASSH_OIDC_CLIENT_ID" \
            "CASSH_OIDC_CLIENT_SECRET=$CASSH_OIDC_CLIENT_SECRET" \
            "CASSH_OIDC_TENANT=$CASSH_OIDC_TENANT" \
            "CASSH_CA_PRIVATE_KEY=$CASSH_CA_PRIVATE_KEY" \
            "CASSH_CERT_VALIDITY_HOURS=${CASSH_CERT_VALIDITY_HOURS:-12}" \
            "CASSH_GITHUB_PRINCIPAL_SOURCE=${CASSH_GITHUB_PRINCIPAL_SOURCE:-email_prefix}" \
        --output none
fi

# Get the service URL
SERVICE_URL=$(az containerapp show --name "$SERVICE_NAME" --resource-group "$AZURE_RESOURCE_GROUP" --query 'properties.configuration.ingress.fqdn' --output tsv)
SERVICE_URL="https://${SERVICE_URL}"

# Update with correct server URL
echo -e "${YELLOW}Updating service with OAuth redirect URL...${NC}"
az containerapp update \
    --name "$SERVICE_NAME" \
    --resource-group "$AZURE_RESOURCE_GROUP" \
    --set-env-vars \
        "CASSH_SERVER_URL=$SERVICE_URL" \
        "CASSH_OIDC_REDIRECT_URL=${SERVICE_URL}/auth/callback" \
    --output none

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
echo "  View logs:     az containerapp logs show --name $SERVICE_NAME --resource-group $AZURE_RESOURCE_GROUP"
echo "  View app:      az containerapp show --name $SERVICE_NAME --resource-group $AZURE_RESOURCE_GROUP"
echo "  Delete app:    az containerapp delete --name $SERVICE_NAME --resource-group $AZURE_RESOURCE_GROUP"
echo "  Delete all:    az group delete --name $AZURE_RESOURCE_GROUP"
