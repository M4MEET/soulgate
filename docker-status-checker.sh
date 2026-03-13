#!/bin/bash
# Docker Status Checker Script

# Log file path
docker_log="/Users/demon/soulGate/docker_status.log"

# Check Docker status and append to log file
docker ps -a >> "$docker_log"

# Check for stopped or exited containers
stopped_containers=$(docker ps -a --filter "status=exited" --filter "status=created" --format "table {{.Names}}\t{{.Status}}")

# Notify user if there are stopped or exited containers
if [[ -n "$stopped_containers" ]]; then
  echo "Stopped or exited containers detected:" >> "$docker_log"
  echo "$stopped_containers" >> "$docker_log"
  echo -e "\nStopped or exited containers:\n$stopped_containers"
fi
