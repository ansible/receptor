#!/bin/bash

echo Warning: There is only one image per repo, not per branch, so rebuilding
echo the CI runner image affects all CI jobs across the repo.

read -p "GitHub Username: " USERNAME
read -p "GitHub OTP Code: " GITHUB_OTP
read -p "Fork/Repo [project-receptor/receptor]: " REPO
REPO=${REPO:-project-receptor/receptor}
read -p "Branch or ref [devel]: " BRANCH
BRANCH=${BRANCH:-devel}

curl -i \
  -u $USERNAME \
  -X POST \
  -H "Accept: application/vnd.github.v3+json" \
  -H "x-github-otp: $GITHUB_OTP" \
  https://api.github.com/repos/$REPO/actions/workflows/daily.yml/dispatches \
  -d "{\"ref\":\"$BRANCH\"}"
