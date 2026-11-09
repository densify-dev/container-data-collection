#!/usr/bin/env bash
# Opens (or updates) a helm-charts PR that moves the
# container-optimization-data-forwarder chart to a newly released image:
# creates a branch on helm-charts from master, checks it out under
# RUNNER_TEMP, patch-bumps the chart version, sets
# images.dataCollection.specificTag and images.dataCollection.floatingTag
# in values.yaml and in the README.md values table, then commits, pushes
# and opens the PR.
#
# Env vars:
#   TAG                - required, released image tag (e.g. 4.9.0)
#   MAJOR_VERSION      - required, major version of TAG (e.g. 4)
#   GH_TOKEN           - required, app token with write access to CHARTS_REPO
#   APP_SLUG           - required, GitHub App slug used as the commit author
#   CHARTS_REPO        - required, e.g. densify-dev/helm-charts
#   CHART_DIR          - required, chart path within CHARTS_REPO
#   RUNNER_TEMP, GITHUB_REPOSITORY, GITHUB_SERVER_URL, GITHUB_RUN_ID
#                      - required (set by the Actions runner)
set -euo pipefail

: "${TAG:?TAG must be set}"
: "${MAJOR_VERSION:?MAJOR_VERSION must be set}"
: "${GH_TOKEN:?GH_TOKEN must be set}"
: "${APP_SLUG:?APP_SLUG must be set}"
: "${CHARTS_REPO:?CHARTS_REPO must be set}"
: "${CHART_DIR:?CHART_DIR must be set}"
: "${RUNNER_TEMP:?RUNNER_TEMP must be set}"
: "${GITHUB_REPOSITORY:?GITHUB_REPOSITORY must be set}"
: "${GITHUB_SERVER_URL:?GITHUB_SERVER_URL must be set}"
: "${GITHUB_RUN_ID:?GITHUB_RUN_ID must be set}"

branch="update/container-optimization-data-forwarder-${TAG}"
work_dir="${RUNNER_TEMP}/helm-charts"

# Sets images.dataCollection.<key> in values.yaml to "<value>".
set_values_key() {
  local key="$1" value="$2"
  sed -i -E "/^  dataCollection:/,/^  [^ ]/ s/^(    ${key}: ).*/\1\"${value}\"/" values.yaml
  grep -q "^    ${key}: \"${value}\"\$" values.yaml
}

# Sets the default-value column of the images.dataCollection.<key> row in README.md.
set_readme_default() {
  local key="$1" value="$2"
  sed -i -E "/^\| \`images\.dataCollection\.${key}\`/ s/\`[^\`]*\` \|\$/\`${value}\` |/" README.md
  grep -q "^| \`images\.dataCollection\.${key}\`.*| \`${value}\` |\$" README.md
}

# Create the branch on helm-charts from master (reset it to master if a
# previous run for this tag already created it).
master_sha="$(gh api "repos/${CHARTS_REPO}/git/ref/heads/master" --jq .object.sha)"
if gh api "repos/${CHARTS_REPO}/git/ref/heads/${branch}" > /dev/null 2>&1; then
  gh api -X PATCH "repos/${CHARTS_REPO}/git/refs/heads/${branch}" -f sha="${master_sha}" -F force=true > /dev/null
else
  gh api "repos/${CHARTS_REPO}/git/refs" -f ref="refs/heads/${branch}" -f sha="${master_sha}" > /dev/null
fi

# Check the branch out outside this repo's workspace
git clone --quiet --depth 1 --branch "${branch}" \
  "https://x-access-token:${GH_TOKEN}@github.com/${CHARTS_REPO}.git" "${work_dir}"
cd "${work_dir}/${CHART_DIR}"

current_version="$(sed -n -E 's/^version: *//p' Chart.yaml)"
if [[ ! "${current_version}" =~ ^([0-9]+)\.([0-9]+)\.([0-9]+)$ ]]; then
  echo "::error::unexpected chart version format: ${current_version}"
  exit 1
fi
next_version="${BASH_REMATCH[1]}.${BASH_REMATCH[2]}.$((BASH_REMATCH[3] + 1))"
sed -i -E "s/^version: .*/version: ${next_version}/" Chart.yaml
grep -q "^version: ${next_version}\$" Chart.yaml

set_values_key specificTag "${TAG}"
set_values_key floatingTag "${MAJOR_VERSION}"
set_readme_default specificTag "${TAG}"
set_readme_default floatingTag "${MAJOR_VERSION}"

changed="$(git diff --name-only | sort | tr '\n' ' ')"
expected="${CHART_DIR}/Chart.yaml ${CHART_DIR}/README.md ${CHART_DIR}/values.yaml "
if [[ "${changed}" != "${expected}" ]]; then
  echo "::error::unexpected file set changed: ${changed}"
  exit 1
fi

bot_id="$(gh api "/users/${APP_SLUG}[bot]" --jq .id)"
git config user.name "${APP_SLUG}[bot]"
git config user.email "${bot_id}+${APP_SLUG}[bot]@users.noreply.github.com"
git commit --quiet -am "Update container-optimization-data-forwarder image to ${TAG}"
git push --quiet origin "HEAD:${branch}"

title="Update container-optimization-data-forwarder image to ${TAG}"
body_file="$(mktemp)"
{
  echo "This PR was created automatically by the ${GITHUB_REPOSITORY} release workflow after image ${TAG} was built, pushed and passed the Trivy and Docker Scout scans."
  echo
  echo "Changes:"
  echo "- Set \`images.dataCollection.specificTag\` to \`${TAG}\` in values.yaml and README.md"
  echo "- Set \`images.dataCollection.floatingTag\` to \`${MAJOR_VERSION}\` in values.yaml and README.md"
  echo "- Bumped container-optimization-data-forwarder chart to ${next_version}"
  echo
  echo "Workflow run: ${GITHUB_SERVER_URL}/${GITHUB_REPOSITORY}/actions/runs/${GITHUB_RUN_ID}"
} > "${body_file}"

existing="$(gh pr list --repo "${CHARTS_REPO}" --head "${branch}" --state open --json number --jq '.[0].number // empty')"
if [[ -n "${existing}" ]]; then
  gh pr edit "${existing}" --repo "${CHARTS_REPO}" --title "${title}" --body-file "${body_file}"
  echo "updated PR #${existing}"
else
  gh pr create --repo "${CHARTS_REPO}" --base master --head "${branch}" --title "${title}" --body-file "${body_file}"
fi
