#!/bin/bash
# deploy-aws.sh - Deploy cassh to AWS App Runner
#
# Prerequisites:
#   1. Install AWS CLI v2: https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html
#   2. Configure credentials: aws configure
#   3. Install Docker: https://docs.docker.com/get-docker/
#
# Usage:
#   ./scripts/deploy-aws.sh
#
# Required environment variables (set these before running):
#   CASSH_OIDC_CLIENT_ID      - Microsoft Entra app client ID
#   CASSH_OIDC_CLIENT_SECRET  - Microsoft Entra app client secret
#   CASSH_OIDC_TENANT         - Microsoft Entra tenant ID
#   CASSH_CA_PRIVATE_KEY      - CA private key (full PEM content)
#
# Optional environment variables:
#   AWS_REGION                - AWS region (defaults to us-east-1)
#   SERVICE_NAME              - App Runner service name (defaults to cassh)

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}🚀 Deploying cassh to AWS App Runner${NC}"
echo ""

# Check for AWS CLI
if ! command -v aws &> /dev/null; then
    echo -e "${RED}Error: AWS CLI not found${NC}"
    echo "Install it from: https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html"
    exit 1
fi

# Check for Docker
if ! command -v docker &> /dev/null; then
    echo -e "${RED}Error: Docker not found${NC}"
    echo "Install it from: https://docs.docker.com/get-docker/"
    exit 1
fi

# Configuration
AWS_REGION=${AWS_REGION:-us-east-1}
SERVICE_NAME=${SERVICE_NAME:-cassh}
AWS_ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
ECR_REPO="${AWS_ACCOUNT_ID}.dkr.ecr.${AWS_REGION}.amazonaws.com/${SERVICE_NAME}"

echo -e "${YELLOW}Configuration:${NC}"
echo "  Account:  $AWS_ACCOUNT_ID"
echo "  Region:   $AWS_REGION"
echo "  Service:  $SERVICE_NAME"
echo "  ECR Repo: $ECR_REPO"
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

# Create ECR repository if it doesn't exist
echo -e "${YELLOW}Creating ECR repository...${NC}"
aws ecr describe-repositories --repository-names "$SERVICE_NAME" --region "$AWS_REGION" 2>/dev/null || \
    aws ecr create-repository --repository-name "$SERVICE_NAME" --region "$AWS_REGION"

# Login to ECR
echo -e "${YELLOW}Logging into ECR...${NC}"
aws ecr get-login-password --region "$AWS_REGION" | docker login --username AWS --password-stdin "$AWS_ACCOUNT_ID.dkr.ecr.$AWS_REGION.amazonaws.com"

# Build and push image
echo -e "${YELLOW}Building and pushing Docker image...${NC}"
cd "$(dirname "$0")/.."

docker build -t "$SERVICE_NAME" .
docker tag "$SERVICE_NAME:latest" "$ECR_REPO:latest"
docker push "$ECR_REPO:latest"

# Create App Runner access role if it doesn't exist
ROLE_NAME="cassh-apprunner-ecr-access"
echo -e "${YELLOW}Setting up IAM role for App Runner...${NC}"

# Check if role exists
if ! aws iam get-role --role-name "$ROLE_NAME" 2>/dev/null; then
    # Create the role
    cat > /tmp/trust-policy.json << 'EOF'
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Principal": {
                "Service": "build.apprunner.amazonaws.com"
            },
            "Action": "sts:AssumeRole"
        }
    ]
}
EOF
    aws iam create-role --role-name "$ROLE_NAME" --assume-role-policy-document file:///tmp/trust-policy.json
    aws iam attach-role-policy --role-name "$ROLE_NAME" --policy-arn arn:aws:iam::aws:policy/service-role/AWSAppRunnerServicePolicyForECRAccess
    rm /tmp/trust-policy.json

    # Wait for role to propagate
    echo "Waiting for IAM role to propagate..."
    sleep 10
fi

ACCESS_ROLE_ARN="arn:aws:iam::${AWS_ACCOUNT_ID}:role/${ROLE_NAME}"

# Check if service already exists
echo -e "${YELLOW}Deploying to App Runner...${NC}"
EXISTING_SERVICE=$(aws apprunner list-services --region "$AWS_REGION" --query "ServiceSummaryList[?ServiceName=='$SERVICE_NAME'].ServiceArn" --output text)

if [ -n "$EXISTING_SERVICE" ]; then
    # Update existing service
    echo "Updating existing App Runner service..."
    aws apprunner update-service \
        --service-arn "$EXISTING_SERVICE" \
        --source-configuration "{
            \"ImageRepository\": {
                \"ImageIdentifier\": \"$ECR_REPO:latest\",
                \"ImageRepositoryType\": \"ECR\",
                \"ImageConfiguration\": {
                    \"Port\": \"8080\",
                    \"RuntimeEnvironmentVariables\": {
                        \"CASSH_OIDC_CLIENT_ID\": \"$CASSH_OIDC_CLIENT_ID\",
                        \"CASSH_OIDC_CLIENT_SECRET\": \"$CASSH_OIDC_CLIENT_SECRET\",
                        \"CASSH_OIDC_TENANT\": \"$CASSH_OIDC_TENANT\",
                        \"CASSH_CA_PRIVATE_KEY\": \"$CASSH_CA_PRIVATE_KEY\",
                        \"CASSH_CERT_VALIDITY_HOURS\": \"${CASSH_CERT_VALIDITY_HOURS:-12}\",
                        \"CASSH_GITHUB_PRINCIPAL_SOURCE\": \"${CASSH_GITHUB_PRINCIPAL_SOURCE:-email_prefix}\"
                    }
                }
            },
            \"AuthenticationConfiguration\": {
                \"AccessRoleArn\": \"$ACCESS_ROLE_ARN\"
            }
        }" \
        --region "$AWS_REGION"

    SERVICE_ARN="$EXISTING_SERVICE"
else
    # Create new service
    echo "Creating new App Runner service..."
    SERVICE_ARN=$(aws apprunner create-service \
        --service-name "$SERVICE_NAME" \
        --source-configuration "{
            \"ImageRepository\": {
                \"ImageIdentifier\": \"$ECR_REPO:latest\",
                \"ImageRepositoryType\": \"ECR\",
                \"ImageConfiguration\": {
                    \"Port\": \"8080\",
                    \"RuntimeEnvironmentVariables\": {
                        \"CASSH_OIDC_CLIENT_ID\": \"$CASSH_OIDC_CLIENT_ID\",
                        \"CASSH_OIDC_CLIENT_SECRET\": \"$CASSH_OIDC_CLIENT_SECRET\",
                        \"CASSH_OIDC_TENANT\": \"$CASSH_OIDC_TENANT\",
                        \"CASSH_CA_PRIVATE_KEY\": \"$CASSH_CA_PRIVATE_KEY\",
                        \"CASSH_CERT_VALIDITY_HOURS\": \"${CASSH_CERT_VALIDITY_HOURS:-12}\",
                        \"CASSH_GITHUB_PRINCIPAL_SOURCE\": \"${CASSH_GITHUB_PRINCIPAL_SOURCE:-email_prefix}\"
                    }
                }
            },
            \"AuthenticationConfiguration\": {
                \"AccessRoleArn\": \"$ACCESS_ROLE_ARN\"
            }
        }" \
        --instance-configuration "{
            \"Cpu\": \"0.25 vCPU\",
            \"Memory\": \"0.5 GB\"
        }" \
        --health-check-configuration "{
            \"Protocol\": \"HTTP\",
            \"Path\": \"/health\",
            \"Interval\": 10,
            \"Timeout\": 5,
            \"HealthyThreshold\": 1,
            \"UnhealthyThreshold\": 5
        }" \
        --region "$AWS_REGION" \
        --query 'Service.ServiceArn' \
        --output text)
fi

# Wait for deployment
echo -e "${YELLOW}Waiting for deployment to complete...${NC}"
echo "This may take a few minutes..."

while true; do
    STATUS=$(aws apprunner describe-service --service-arn "$SERVICE_ARN" --region "$AWS_REGION" --query 'Service.Status' --output text)
    if [ "$STATUS" = "RUNNING" ]; then
        break
    elif [ "$STATUS" = "CREATE_FAILED" ] || [ "$STATUS" = "DELETE_FAILED" ]; then
        echo -e "${RED}Error: Deployment failed with status: $STATUS${NC}"
        exit 1
    fi
    echo "  Status: $STATUS"
    sleep 10
done

# Get the service URL
SERVICE_URL=$(aws apprunner describe-service --service-arn "$SERVICE_ARN" --region "$AWS_REGION" --query 'Service.ServiceUrl' --output text)
SERVICE_URL="https://${SERVICE_URL}"

# Update with correct server URL
echo -e "${YELLOW}Updating service with OAuth redirect URL...${NC}"
aws apprunner update-service \
    --service-arn "$SERVICE_ARN" \
    --source-configuration "{
        \"ImageRepository\": {
            \"ImageIdentifier\": \"$ECR_REPO:latest\",
            \"ImageRepositoryType\": \"ECR\",
            \"ImageConfiguration\": {
                \"Port\": \"8080\",
                \"RuntimeEnvironmentVariables\": {
                    \"CASSH_OIDC_CLIENT_ID\": \"$CASSH_OIDC_CLIENT_ID\",
                    \"CASSH_OIDC_CLIENT_SECRET\": \"$CASSH_OIDC_CLIENT_SECRET\",
                    \"CASSH_OIDC_TENANT\": \"$CASSH_OIDC_TENANT\",
                    \"CASSH_CA_PRIVATE_KEY\": \"$CASSH_CA_PRIVATE_KEY\",
                    \"CASSH_CERT_VALIDITY_HOURS\": \"${CASSH_CERT_VALIDITY_HOURS:-12}\",
                    \"CASSH_GITHUB_PRINCIPAL_SOURCE\": \"${CASSH_GITHUB_PRINCIPAL_SOURCE:-email_prefix}\",
                    \"CASSH_SERVER_URL\": \"$SERVICE_URL\",
                    \"CASSH_OIDC_REDIRECT_URL\": \"${SERVICE_URL}/auth/callback\"
                }
            }
        },
        \"AuthenticationConfiguration\": {
            \"AccessRoleArn\": \"$ACCESS_ROLE_ARN\"
        }
    }" \
    --region "$AWS_REGION" > /dev/null

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
echo "  View logs:     aws apprunner list-operations --service-arn $SERVICE_ARN --region $AWS_REGION"
echo "  View service:  aws apprunner describe-service --service-arn $SERVICE_ARN --region $AWS_REGION"
echo "  Delete:        aws apprunner delete-service --service-arn $SERVICE_ARN --region $AWS_REGION"
