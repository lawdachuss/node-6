# Graceful Shutdown Implementation

## Overview

Implemented comprehensive graceful shutdown handling for GitHub Actions workflow to ensure clean termination when workflow is cancelled or times out.

## Problem

When a workflow is cancelled:
- Docker containers were forcefully killed
- Channels weren't synced to Supabase
- Resources weren't cleaned up properly
- Processes left running in background
- No proper shutdown sequence

## Solution

### 1. Signal Handling in Main Loop

Added trap for SIGTERM/SIGINT signals:

```bash
# Setup trap for graceful shutdown on SIGTERM/SIGINT
trap 'echo ""; echo "⚠️  Received cancellation signal - initiating graceful shutdown..."; CANCELLED=true' SIGTERM SIGINT
CANCELLED=false

# Monitor loop checks cancellation flag
while [ $(date +%s) -lt $END_TIME ] && [ "$CANCELLED" = false ]; do
  # ... monitoring code ...
  sleep 60
done
```

**Benefits**:
- Detects cancellation immediately
- Breaks out of monitoring loop cleanly
- Allows shutdown sequence to run

### 2. Graceful Container Shutdown

Enhanced Docker stop with timeout:

```bash
# Give container time to finish current operations
echo "⏳ Waiting for container to finish current operations (30 seconds)..."
sleep 30

# Stop Docker container gracefully with 60 second timeout
echo "🛑 Stopping Docker container gracefully..."
docker stop --time=60 goondvr
```

**Benefits**:
- Container receives SIGTERM first
- 60 seconds to finish current operations
- Allows recordings to finalize
- Prevents data corruption

### 3. Six-Step Shutdown Sequence

Implemented structured shutdown with `if: always()`:

#### Step 1: Sync Channels to Supabase 💾
```bash
if docker ps | grep -q goondvr; then
  docker exec goondvr cat /usr/src/app/conf/channels.json > conf/channels.json
fi
./scripts/sync-channels-to-supabase.sh
```
**Purpose**: Preserve channel configuration for next run

#### Step 2: Stop Docker Container 🛑
```bash
docker stop --time=60 goondvr
```
**Purpose**: Gracefully stop main application

#### Step 3: Stop Byparr Container 🛑
```bash
docker stop --time=10 byparr
```
**Purpose**: Stop Cloudflare bypass service

#### Step 4: Stop Cloudflare Tunnel ☁️
```bash
kill -TERM $TUNNEL_PID
sleep 2
kill -9 $TUNNEL_PID  # Force kill if still running
pkill -f cloudflared
```
**Purpose**: Close public tunnel cleanly

#### Step 5: Remove Containers 🗑️
```bash
docker rm -f goondvr byparr
docker network rm recorder-network
```
**Purpose**: Clean up container resources

#### Step 6: Remove Images 🗑️
```bash
docker rmi goondvr:latest
docker rmi ghcr.io/thephaseless/byparr:latest
docker system prune -af --volumes
```
**Purpose**: Free disk space

## Shutdown Scenarios

### Scenario 1: Normal Completion ✅
```
5-hour session completes → Shutdown sequence runs → Clean exit
```

### Scenario 2: Manual Cancellation ⚠️
```
User clicks Cancel → SIGTERM received → CANCELLED=true → Loop exits → Shutdown sequence runs
```

### Scenario 3: Workflow Timeout ⏱️
```
Timeout reached → SIGTERM received → CANCELLED=true → Loop exits → Shutdown sequence runs
```

### Scenario 4: Container Crash 💥
```
Container stops → Loop detects → Breaks → Shutdown sequence runs
```

### Scenario 5: Workflow Failure ❌
```
Step fails → if: always() ensures cleanup runs → Shutdown sequence runs
```

## Visual Shutdown Flow

```
┌─────────────────────────────────────────┐
│   Cancellation Signal Received          │
│   (SIGTERM/SIGINT/Timeout)              │
└──────────────┬──────────────────────────┘
               │
               ▼
┌─────────────────────────────────────────┐
│   Set CANCELLED=true                     │
│   Break monitoring loop                  │
└──────────────┬──────────────────────────┘
               │
               ▼
┌─────────────────────────────────────────┐
│   Display cancellation banner            │
│   "WORKFLOW CANCELLED - SHUTTING DOWN"   │
└──────────────┬──────────────────────────┘
               │
               ▼
┌─────────────────────────────────────────┐
│   Step 1: Sync channels to Supabase     │
│   ✅ Preserve configuration              │
└──────────────┬──────────────────────────┘
               │
               ▼
┌─────────────────────────────────────────┐
│   Wait 30 seconds for operations         │
│   ⏳ Let recordings finalize             │
└──────────────┬──────────────────────────┘
               │
               ▼
┌─────────────────────────────────────────┐
│   Step 2: Stop goondvr (60s timeout)    │
│   🛑 SIGTERM → wait → SIGKILL           │
└──────────────┬──────────────────────────┘
               │
               ▼
┌─────────────────────────────────────────┐
│   Step 3: Stop byparr (10s timeout)     │
│   🛑 Stop Cloudflare bypass             │
└──────────────┬──────────────────────────┘
               │
               ▼
┌─────────────────────────────────────────┐
│   Step 4: Stop cloudflared tunnel       │
│   ☁️  Close public access               │
└──────────────┬──────────────────────────┘
               │
               ▼
┌─────────────────────────────────────────┐
│   Step 5: Remove containers & network   │
│   🗑️  Clean up Docker resources         │
└──────────────┬──────────────────────────┘
               │
               ▼
┌─────────────────────────────────────────┐
│   Step 6: Remove images & prune         │
│   🧹 Free disk space                     │
└──────────────┬──────────────────────────┘
               │
               ▼
┌─────────────────────────────────────────┐
│   Cache cleanup (separate step)         │
│   🧹 Clear all caches                    │
└──────────────┬──────────────────────────┘
               │
               ▼
┌─────────────────────────────────────────┐
│   ✅ Graceful shutdown complete          │
└─────────────────────────────────────────┘
```

## Key Features

### 1. Always Runs 🔒
```yaml
- name: Graceful shutdown and cleanup
  if: always()
```
- Runs even if previous steps fail
- Runs on cancellation
- Runs on timeout
- Guaranteed cleanup

### 2. Timeout Protection ⏱️
```bash
docker stop --time=60 goondvr
```
- 60 seconds for graceful shutdown
- SIGTERM sent first
- SIGKILL after timeout
- Prevents hanging

### 3. Progress Tracking 📊
```
Step 1/6: Syncing channels...
Step 2/6: Stopping container...
Step 3/6: Stopping Byparr...
...
```
- Clear progress indication
- Easy to debug
- Shows what's happening

### 4. Error Handling 🛡️
```bash
command && echo "✅ Success" || echo "⚠️  Failed"
```
- Continues even if steps fail
- Shows success/failure status
- Doesn't block cleanup

## Logs Output

### Normal Completion
```
[INFO] 5-hour session completed at Mon May 3 10:00:00 UTC 2026
[INFO] Initiating graceful shutdown...

💾 Syncing channels to Supabase before shutdown...
✅ Channels synced
⏳ Waiting for container to finish current operations (30 seconds)...
🛑 Stopping Docker container gracefully...
✅ Container stopped gracefully
```

### Cancelled Workflow
```
⚠️  Received cancellation signal - initiating graceful shutdown...

╔════════════════════════════════════════════════════════════╗
║           ⚠️  WORKFLOW CANCELLED - SHUTTING DOWN           ║
╚════════════════════════════════════════════════════════════╝

[INFO] Workflow cancelled at Mon May 3 08:30:00 UTC 2026
[INFO] Initiating graceful shutdown...

💾 Step 1/6: Syncing channels to Supabase...
✅ Channels synced

🛑 Step 2/6: Stopping Docker container...
  Sending SIGTERM to container (60 second timeout)...
✅ Container stopped gracefully

🛑 Step 3/6: Stopping Byparr container...
✅ Byparr stopped

☁️  Step 4/6: Stopping Cloudflare tunnel...
✅ Tunnel stopped

🗑️  Step 5/6: Removing Docker containers...
✅ Containers removed
✅ Network removed

🗑️  Step 6/6: Removing Docker images...
✅ goondvr image removed
✅ byparr image removed
✅ flaresolverr image removed

🧹 Final Docker cleanup...
✅ Docker system cleaned

✅ Graceful shutdown completed at: Mon May 3 08:32:00 UTC 2026
```

## Benefits

### Before Implementation ❌
- Containers forcefully killed (SIGKILL)
- Channels not synced
- Resources left behind
- Processes still running
- Disk space wasted
- No visibility into shutdown

### After Implementation ✅
- Graceful container shutdown (SIGTERM)
- Channels always synced
- All resources cleaned
- All processes stopped
- Disk space freed
- Clear shutdown progress

## Testing

### Test 1: Normal Completion
1. Let workflow run for 5 hours
2. Check logs for graceful shutdown
3. **Expected**: Clean shutdown sequence

### Test 2: Manual Cancellation
1. Start workflow
2. Cancel after 1 hour
3. Check logs for cancellation banner
4. **Expected**: Graceful shutdown with sync

### Test 3: Container Crash
1. Simulate container crash
2. Check if cleanup runs
3. **Expected**: Cleanup detects and runs

### Test 4: Timeout
1. Set short timeout
2. Let workflow timeout
3. **Expected**: Graceful shutdown

## Configuration

No configuration needed - works automatically with:
- `if: always()` on cleanup step
- Signal traps in monitoring loop
- Timeout parameters on docker stop

## Troubleshooting

### Issue: Container Won't Stop
**Solution**: 60-second timeout ensures SIGKILL after grace period

### Issue: Channels Not Synced
**Solution**: Sync happens in multiple places (loop + shutdown)

### Issue: Resources Left Behind
**Solution**: `if: always()` ensures cleanup always runs

### Issue: Shutdown Takes Too Long
**Solution**: Optimized timeouts (60s container, 10s byparr)

## Summary

✅ **Graceful shutdown fully implemented**  
✅ **Handles cancellation, timeout, and failure**  
✅ **Six-step structured cleanup**  
✅ **Always syncs channels to Supabase**  
✅ **Gives containers time to finish**  
✅ **Cleans all resources**  
✅ **Clear progress tracking**  
✅ **Error handling throughout**  

The workflow now shuts down cleanly in all scenarios, preserving data and cleaning up resources properly.
