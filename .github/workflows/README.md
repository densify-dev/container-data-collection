# GitHub Actions Docker Build

This repository includes a GitHub Actions workflow that automatically builds and pushes Docker containers when a new tag is created.

## How it works

The workflow is triggered when you create a new tag in the repository. It will:

1. Build Docker images for both `linux/amd64` and `linux/arm64` platforms
2. Push the images to Docker Hub with tags based on the type of release
3. Scan the built container for security vulnerabilities using Trivy
4. Upload security scan results to GitHub Security tab
5. Fail the build if critical or high severity vulnerabilities are found
6. Generate Software Bill of Materials (SBOM) using Syft in CycloneDX JSON format
7. Upload SBOM as a downloadable artifact

### For all tags:
- `densify/container-optimization-data-forwarder:${tag}`
- `densify/container-optimization-data-forwarder:alpine-${tag}`

### For official releases only (format: v#.#.#):
- `densify/container-optimization-data-forwarder:v${major_version}` (e.g., `v4` for `v4.2.2`)
- `densify/container-optimization-data-forwarder:alpine-v${major_version}` (e.g., `alpine-v4` for `v4.2.2`)

### Official vs Non-Official Releases

**Official releases** follow the pattern `v#.#.#` (e.g., `v4.2.2`, `v1.0.0`) and will also get major version tags.

**Non-official releases** like `v4.2.2-beta1`, `v4.2.2-alpha`, `v4.2.2-rc1` will only get the exact tag, not the major version tag. This ensures that customers pulling `v4` always get the latest stable release, not a pre-release version.

## Setup

### Prerequisites

1. **Docker Hub Account**: You need a Docker Hub account with push permissions to the `densify/container-optimization-data-forwarder` repository.

2. **GitHub Secrets**: Add the following secrets to your GitHub repository:
   - `DOCKER_USERNAME`: Your Docker Hub username
   - `DOCKER_PASSWORD`: Your Docker Hub password or access token

3. **GitHub Permissions**: The workflow requires the following permissions (automatically granted):
   - `contents: read` - To checkout the repository
   - `packages: write` - To push Docker images
   - `security-events: write` - To upload security scan results to GitHub Security tab

### Adding Secrets

1. Go to your GitHub repository
2. Navigate to Settings → Secrets and variables → Actions
3. Click "New repository secret"
4. Add the following secrets:
   - Name: `DOCKER_USERNAME`, Value: Your Docker Hub username
   - Name: `DOCKER_PASSWORD`, Value: Your Docker Hub password or access token

## Usage

### Creating a Release

To trigger a build, create and push a new tag:

```bash
# Official release (will also create major version tags)
git tag v4.2.2
git push origin v4.2.2

# Beta/Alpha release (will only create exact tag)
git tag v4.2.2-beta1
git push origin v4.2.2-beta1
```

### Examples

**Official Release `v4.2.2`** creates these tags:
- `densify/container-optimization-data-forwarder:v4.2.2`
- `densify/container-optimization-data-forwarder:alpine-v4.2.2`
- `densify/container-optimization-data-forwarder:v4` ← Updates major version
- `densify/container-optimization-data-forwarder:alpine-v4` ← Updates major version

**Beta Release `v4.2.2-beta1`** creates only:
- `densify/container-optimization-data-forwarder:v4.2.2-beta1`
- `densify/container-optimization-data-forwarder:alpine-v4.2.2-beta1`

### Build Arguments

The workflow automatically sets the following build arguments:

- `BASE_IMAGE=alpine`: Uses Alpine Linux as the base image
- `VERSION=${tag}`: Sets the version to the Git tag name
- `RELEASE=${SHA}`: Sets the release to the Git commit SHA

### Security Scanning

The workflow includes Trivy vulnerability scanning with two scan steps:

### 1. SARIF Upload
- Scans the built container for all vulnerabilities
- Uploads results to GitHub Security tab in SARIF format
- Always runs, even if build fails
- Provides detailed vulnerability information in GitHub's security interface

### 2. Build Gate
- Scans for CRITICAL and HIGH severity vulnerabilities
- Focuses on OS and library vulnerabilities
- Ignores unfixed vulnerabilities (those without available patches)
- **Fails the build** if critical or high severity vulnerabilities are found
- Shows results in table format in the action logs

### Viewing Security Results

1. **GitHub Security Tab**: Go to your repository → Security → Code scanning alerts
2. **Action Logs**: Check the workflow run logs for the table output
3. **Build Status**: The workflow will fail if high/critical vulnerabilities are found

### Software Bill of Materials (SBOM)

The workflow generates a comprehensive SBOM using Syft:

- **Format**: CycloneDX JSON - industry standard for SBOM interchange
- **Content**: Complete inventory of all packages, libraries, and dependencies
- **Artifact**: Available as downloadable artifact named `sbom-${tag}`
- **Retention**: SBOM artifacts are kept for 90 days
- **Use Cases**: 
  - Supply chain security analysis
  - License compliance tracking
  - Vulnerability management
  - Dependency auditing

### Downloading SBOM

1. Go to the completed workflow run in GitHub Actions
2. Scroll down to the "Artifacts" section
3. Download the `sbom-${tag}` artifact (e.g., `sbom-v4.2.2`)
4. Extract the ZIP file to access `sbom.cyclonedx.json`

## Multi-Platform Support

The workflow uses Docker Buildx to build images for both:
- `linux/amd64` (Intel/AMD 64-bit)
- `linux/arm64` (ARM 64-bit, including Apple Silicon)

## Dockerfile Changes

The Dockerfile has been updated to support multi-platform builds by:

1. Adding `ARG TARGETARCH` to access the target architecture
2. Changing the Go build command from `GOARCH=amd64` to `GOARCH=${TARGETARCH}`

This allows the same Dockerfile to build for different architectures automatically.

## Monitoring Builds

You can monitor the build progress by:

1. Going to the "Actions" tab in your GitHub repository
2. Clicking on the "Build and Push Docker Images" workflow
3. Viewing the logs for each step

## Troubleshooting

### Build Failures

If the build fails, check:

1. **Docker Hub credentials**: Ensure `DOCKER_USERNAME` and `DOCKER_PASSWORD` secrets are correctly set
2. **Docker Hub permissions**: Verify you have push permissions to the repository
3. **Security vulnerabilities**: Check if Trivy found critical or high severity vulnerabilities
4. **Build logs**: Check the GitHub Actions logs for specific error messages

### SBOM Generation

The SBOM (Software Bill of Materials) provides:

- **Complete Package Inventory**: All OS packages, language libraries, and dependencies
- **License Information**: License details for compliance tracking
- **Version Details**: Exact versions of all components
- **Vulnerability Correlation**: Can be used with security tools for vulnerability tracking
- **Supply Chain Transparency**: Full visibility into software components

### Security Scan Failures

If the security scan fails the build:

1. Review the Trivy scan results in the action logs
2. Check the GitHub Security tab for detailed vulnerability information
3. Update base images or dependencies to fix vulnerabilities
4. Consider using `.trivyignore` file to ignore false positives (use carefully)

### Security Scan Upload Issues

If you see "Resource not accessible by integration" errors:

1. **Check Repository Settings**: Ensure Code scanning is enabled
   - Go to Settings → Code security and analysis
   - Enable "Code scanning" if not already enabled
2. **Verify Permissions**: The workflow needs `security-events: write` permission
   - This is automatically granted by the workflow configuration
3. **Organization Settings**: For organization repositories, ensure security features are enabled at the org level

### Tag Format

The workflow handles different tag formats:

**Official Releases** (will create major version tags):
- `v4.2.2`, `v1.0.0`, `v10.5.3` - Standard semantic versioning

**Pre-releases** (exact tag only, no major version updates):
- `v4.2.2-beta1`, `v4.2.2-alpha`, `v4.2.2-rc1` - Beta, alpha, release candidates
- `v4.2.2-hotfix`, `v4.2.2.1` - Hotfixes or patch releases with additional identifiers

This ensures that major version tags (like `v4`) always point to the latest stable release, never to pre-release versions.
