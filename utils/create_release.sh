#!/usr/bin/env bash

OWNER="darthfork"
REPO="promgithub"
RELEASE_NAME="Release v$VERSION"
RELEASE_DESCRIPTION="Description of the release"
ARTIFACTS=(./build/*)

#GITHUB_API="https://api.github.com"
#
## Create a release
#echo "Creating a new GitHub release..."
#response=$(curl -s -X POST -H "Authorization: token $GITHUB_TOKEN" \
#  -H "Accept: application/vnd.github+json" \
#  -d "{
#    \"tag_name\": \"$VERSION\",
#    \"name\": \"$RELEASE_NAME\",
#    \"body\": \"$RELEASE_DESCRIPTION\",
#    \"draft\": false,
#    \"prerelease\": false
#  }" \
#  "$GITHUB_API/repos/$OWNER/$REPO/releases")
#
## Extract the release ID and upload URL
#release_id=$(echo "$response" | jq -r .id)
#upload_url=$(echo "$response" | jq -r .upload_url | sed -e "s/{?name,label}//")
#
## Check if the release was created successfully
#if [ "$release_id" == "null" ]; then
#  echo "Failed to create release. Response:"
#  echo "$response"
#  exit 1
#else
#  echo "Release created with ID: $release_id"
#fi
#
## Upload each artifact
#for ARTIFACT_PATH in "${ARTIFACTS[@]}"; do
#  if [ -f "$ARTIFACT_PATH" ]; then
#    echo "Uploading artifact: $ARTIFACT_PATH..."
#    artifact_name=$(basename "$ARTIFACT_PATH")
#
#    curl -s -X POST -H "Authorization: token $GITHUB_TOKEN" \
#      -H "Content-Type: application/zip" \
#      --data-binary @"$ARTIFACT_PATH" \
#      "$upload_url?name=$artifact_name"
#
#    if [ $? -eq 0 ]; then
#      echo "Artifact $artifact_name uploaded successfully!"
#    else
#      echo "Failed to upload artifact $artifact_name."
#    fi
#  else
#    echo "File $ARTIFACT_PATH does not exist, skipping."
#  fi
#done
