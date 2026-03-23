# Real-World Testing with Nido VM Orchestration

## Overview

This document describes the **real-world testing strategy** for Aftertalk using **2 interlocutors** on a **real platform**. We use **nido** (the universal VM orchestrator) to create isolated test environments.

## Test Scenario

### Objective

Validate the complete Aftertalk application with **real audio streaming** between 2 participants, testing:

1. **Audio Capture**: WebRTC/WebSocket audio from 2 sources
2. **Real-time Transcription**: STT processing during the call
3. **Minutes Generation**: AI-generated minutes after call completion
4. **End-to-end Latency**: Audio to transcription to minutes
5. **Scalability**: Multiple concurrent 2-person sessions

### Test Participants

```
┌─────────────────────────────────────────────────────────────┐
│                      Test Environment                        │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌──────────────┐                    ┌──────────────┐       │
│  │ Participant 1│                    │ Participant 2│       │
│  │   (Client)   │◄───── Audio ─────►│(Professional)│       │
│  │              │     WebSocket      │              │       │
│  └──────┬───────┘                    └──────┬───────┘       │
│         │                                    │               │
│         └──────────────┬─────────────────────┘               │
│                        │                                    │
│                        ▼                                    │
│              ┌──────────────────┐                          │
│              │  Aftertalk       │                          │
│              │  Application     │                          │
│              │                  │                          │
│              │  ┌────────────┐  │                          │
│              │  │  Session   │  │                          │
│              │  │  Manager   │  │                          │
│              │  └────────────┘  │                          │
│              │                  │                          │
│              │  ┌────────────┐  │                          │
│              │  │    STT     │  │                          │
│              │  │  Service   │  │                          │
│              │  └────────────┘  │                          │
│              │                  │                          │
│              │  ┌────────────┐  │                          │
│              │  │    LLM     │  │                          │
│              │  │  Service   │  │                          │
│              │  └────────────┘  │                          │
│              └──────────────────┘                          │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

## Nido VM Setup

### What is Nido?

Nido is a **universal VM orchestrator** that allows spawning, managing, and testing VMs locally:

```bash
# Nido commands
nido spawn      # Create a new VM
nido start      # Start a stopped VM
nido stop       # Stop a running VM
nido ssh        # SSH into a VM
nido ls         # List all VMs
nido delete     # Delete a VM
nido build      # Build VM from blueprint
```

### VM Architecture

We create **3 VMs** for testing:

1. **VM-1**: Aftertalk Application Server
2. **VM-2**: Participant 1 (Client)
3. **VM-3**: Participant 2 (Professional)

```
┌─────────────────────────────────────────────────────────────┐
│                     Host Machine                             │
│                                                              │
│  ┌────────────────────────────────────────────────────────┐ │
│  │                    Nido VMs                             │ │
│  │                                                         │ │
│  │  ┌───────────┐    ┌───────────┐    ┌───────────┐       │ │
│  │  │   VM-1    │    │   VM-2    │    │   VM-3    │       │ │
│  │  │  Server   │    │  Client   │    │Professional│       │ │
│  │  │  Ubuntu   │    │  Ubuntu   │    │  Ubuntu   │       │ │
│  │  │  2GB RAM  │    │  1GB RAM  │    │  1GB RAM  │       │ │
│  │  │  2 CPUs   │    │  1 CPU    │    │  1 CPU    │       │ │
│  │  │  10GB Disk│    │  5GB Disk │    │  5GB Disk │       │ │
│  │  └─────┬─────┘    └─────┬─────┘    └─────┬─────┘       │ │
│  │        │                │                │              │ │
│  │        └────────────────┼────────────────┘              │ │
│  │                         │                               │ │
│  │                    Private Network                       │ │
│  │                   (192.168.100.0/24)                     │ │
│  └─────────────────────────┬────────────────────────────────┘ │
│                            │                                  │
└────────────────────────────┼──────────────────────────────────┘
                             │
                        Test Network
```

## Automated Setup Script

### Step 1: Spawn VMs

```bash
#!/bin/bash
# scripts/setup_test_vms.sh

set -e

echo "🚀 Setting up Aftertalk test environment with nido..."

# VM Configuration
SERVER_VM="aftertalk-server"
CLIENT_VM="aftertalk-client"
PROFESSIONAL_VM="aftertalk-professional"

# Spawn Server VM
echo "📦 Creating server VM..."
nido spawn \
    --image ubuntu-22.04 \
    --memory 2048 \
    --cpus 2 \
    --user-data scripts/cloud-init-server.yaml \
    $SERVER_VM

# Spawn Client VM
echo "👤 Creating client VM..."
nido spawn \
    --image ubuntu-22.04 \
    --memory 1024 \
    --cpus 1 \
    --user-data scripts/cloud-init-client.yaml \
    $CLIENT_VM

# Spawn Professional VM
echo "👔 Creating professional VM..."
nido spawn \
    --image ubuntu-22.04 \
    --memory 1024 \
    --cpus 1 \
    --user-data scripts/cloud-init-professional.yaml \
    $PROFESSIONAL_VM

echo "✅ All VMs created successfully!"
echo ""
echo "VM Status:"
nido ls
```

### Step 2: Cloud-Init Configuration

**Server VM** (`scripts/cloud-init-server.yaml`):

```yaml
#cloud-config
package_update: true
packages:
  - docker.io
  - curl
  - jq

runcmd:
  # Install Go
  - wget -q https://go.dev/dl/go1.25.linux-amd64.tar.gz
  - tar -C /usr/local -xzf go1.25.linux-amd64.tar.gz
  - echo 'export PATH=$PATH:/usr/local/go/bin' >> /etc/profile
  - export PATH=$PATH:/usr/local/go/bin
  
  # Clone repository
  - git clone https://github.com/Josepavese/aftertalk.git /opt/aftertalk
  - cd /opt/aftertalk
  
  # Build application
  - make build
  
  # Create config
  - |
    cat > /opt/aftertalk/.env << EOF
    DATABASE_PATH=/opt/aftertalk/aftertalk.db
    HTTP_HOST=0.0.0.0
    HTTP_PORT=8080
    WS_PORT=8081
    LOG_LEVEL=info
    LOG_FORMAT=json
    JWT_SECRET=test-secret-do-not-use-in-production
    API_KEY=test-api-key-12345
    STT_PROVIDER=google
    STT_GOOGLE_CREDENTIALS_PATH=/opt/creds.json
    LLM_PROVIDER=openai
    LLM_OPENAI_API_KEY=sk-test-key
    LLM_OPENAI_MODEL=gpt-4
    WEBHOOK_URL=http://192.168.100.4:8080/webhook
    EOF
  
  # Start application
  - |
    cat > /etc/systemd/system/aftertalk.service << EOF
    [Unit]
    Description=Aftertalk Application
    After=network.target
    
    [Service]
    Type=simple
    User=root
    WorkingDirectory=/opt/aftertalk
    ExecStart=/opt/aftertalk/bin/aftertalk
    Restart=always
    Environment=DATABASE_PATH=/opt/aftertalk/aftertalk.db
    
    [Install]
    WantedBy=multi-user.target
    EOF
  
  - systemctl daemon-reload
  - systemctl enable aftertalk
  - systemctl start aftertalk
  
  # Wait for application to start
  - sleep 10
  - until curl -s http://localhost:8080/v1/health; do sleep 2; done
```

### Step 3: Deploy Application

```bash
#!/bin/bash
# scripts/deploy_to_vm.sh

VM_NAME=${1:-"aftertalk-server"}

echo "🚀 Deploying Aftertalk to $VM_NAME..."

# Copy application to VM
nido ssh $VM_NAME "mkdir -p /opt/aftertalk"

# Copy binary and config
tar czf - bin/aftertalk .env migrations/ | nido ssh $VM_NAME "tar xzf - -C /opt/aftertalk"

# Restart service
nido ssh $VM_NAME "systemctl restart aftertalk"

# Wait for health check
echo "⏳ Waiting for application to be ready..."
for i in {1..30}; do
    if nido ssh $VM_NAME "curl -s http://localhost:8080/v1/health" > /dev/null; then
        echo "✅ Application is ready!"
        exit 0
    fi
    sleep 2
done

echo "❌ Application failed to start"
exit 1
```

### Step 4: Run 2-Interlocutor Test

```bash
#!/bin/bash
# scripts/run_interlocutor_test.sh

set -e

SERVER_VM="aftertalk-server"
CLIENT_VM="aftertalk-client"
PROFESSIONAL_VM="aftertalk-professional"

SERVER_IP=$(nido info $SERVER_VM --json | jq -r '.ip')
API_KEY="test-api-key-12345"

echo "🎭 Starting 2-interlocutor test..."
echo "Server IP: $SERVER_IP"

# Create test session
echo "📋 Creating session..."
SESSION_RESPONSE=$(curl -s -X POST http://$SERVER_IP:8080/v1/sessions \
    -H "Content-Type: application/json" \
    -H "X-API-Key: $API_KEY" \
    -d '{
        "participant_count": 2,
        "participants": [
            {"user_id": "client-user-001", "role": "client"},
            {"user_id": "professional-user-001", "role": "professional"}
        ]
    }')

SESSION_ID=$(echo $SESSION_RESPONSE | jq -r '.session_id')
CLIENT_TOKEN=$(echo $SESSION_RESPONSE | jq -r '.participants[0].token')
PROFESSIONAL_TOKEN=$(echo $SESSION_RESPONSE | jq -r '.participants[1].token')

echo "✅ Session created: $SESSION_ID"

# Start audio streaming on Client VM
echo "🎤 Starting audio stream on client VM..."
nido ssh $CLIENT_VM "
    cd /opt/aftertalk-tests
    ./audio_streamer.sh \
        --server $SERVER_IP:8080 \
        --token $CLIENT_TOKEN \
        --session $SESSION_ID \
        --role client \
        --duration 300 &
" &

# Start audio streaming on Professional VM
echo "🎤 Starting audio stream on professional VM..."
nido ssh $PROFESSIONAL_VM "
    cd /opt/aftertalk-tests
    ./audio_streamer.sh \
        --server $SERVER_IP:8080 \
        --token $PROFESSIONAL_TOKEN \
        --session $SESSION_ID \
        --role professional \
        --duration 300 &
" &

# Wait for call to complete
echo "⏳ Waiting for 5-minute call to complete..."
sleep 300

# Verify transcription
echo "📝 Checking transcription..."
TRANSCRIPTIONS=$(curl -s http://$SERVER_IP:8080/v1/transcriptions?session_id=$SESSION_ID \
    -H "X-API-Key: $API_KEY")

echo "Transcriptions: $(echo $TRANSCRIPTIONS | jq '. | length') segments"

# Generate minutes
echo "🤖 Generating minutes..."
MINUTES_RESPONSE=$(curl -s -X PUT http://$SERVER_IP:8080/v1/minutes/$SESSION_ID \
    -H "Content-Type: application/json" \
    -H "X-API-Key: $API_KEY")

echo "Minutes generated: $(echo $MINUTES_RESPONSE | jq -r '.id')"

# Verify minutes
echo "📄 Verifying minutes..."
MINUTES=$(curl -s http://$SERVER_IP:8080/v1/minutes/$SESSION_ID \
    -H "X-API-Key: $API_KEY")

echo "Minutes content preview:"
echo $MINUTES | jq '.themes'

# Check webhook delivery
echo "🔔 Checking webhook delivery..."
WEBHOOK_LOGS=$(nido ssh $CLIENT_VM "cat /tmp/webhook_server.log | tail -20")
echo "Webhook delivery: $WEBHOOK_LOGS"

echo "✅ 2-interlocutor test completed successfully!"
```

## Test Scenarios

### Scenario 1: Basic Conversation (5 minutes)

```bash
#!/bin/bash
# scenarios/basic_conversation.sh

DURATION=300  # 5 minutes

run_test "Basic Conversation" $DURATION \
    --participants 2 \
    --audio-type "conversation" \
    --verify-transcription \
    --verify-minutes
```

### Scenario 2: Long Session (1 hour)

```bash
#!/bin/bash
# scenarios/long_session.sh

DURATION=3600  # 1 hour

run_test "Long Session" $DURATION \
    --participants 2 \
    --audio-type "conversation" \
    --verify-transcription \
    --verify-minutes \
    --monitor-memory \
    --monitor-cpu
```

### Scenario 3: Concurrent Sessions

```bash
#!/bin/bash
# scenarios/concurrent_sessions.sh

SESSION_COUNT=10

for i in $(seq 1 $SESSION_COUNT); do
    run_test "Concurrent Session $i" 300 \
        --participants 2 \
        --audio-type "conversation" &
done

wait

echo "✅ All $SESSION_COUNT concurrent sessions completed"
```

### Scenario 4: Network Stress Test

```bash
#!/bin/bash
# scenarios/network_stress.sh

# Simulate packet loss on network interface
nido ssh $SERVER_VM "tc qdisc add dev eth0 root netem loss 1%"

run_test "Network Stress (1% loss)" 300 \
    --participants 2 \
    --audio-type "conversation"

# Remove packet loss
nido ssh $SERVER_VM "tc qdisc del dev eth0 root"
```

### Scenario 5: Audio Quality Variations

```bash
#!/bin/bash
# scenarios/audio_quality.sh

SAMPLE_RATES=(8000 16000 22050 44100 48000)
BIT_DEPTHS=(8 16 24 32)

for rate in "${SAMPLE_RATES[@]}"; do
    for depth in "${BIT_DEPTHS[@]}"; do
        run_test "Audio Quality ${rate}Hz/${depth}bit" 60 \
            --sample-rate $rate \
            --bit-depth $depth \
            --participants 2
    done
done
```

## Automated Test Runner

```bash
#!/bin/bash
# run_all_scenarios.sh

set -e

REPORT_FILE="test_report_$(date +%Y%m%d_%H%M%S).md"

echo "# Aftertalk Real-World Test Report" > $REPORT_FILE
echo "Date: $(date)" >> $REPORT_FILE
echo "" >> $REPORT_FILE

# Setup VMs
echo "🔧 Setting up VMs..."
./scripts/setup_test_vms.sh

# Run scenarios
SCENARIOS=(
    "scenarios/basic_conversation.sh"
    "scenarios/long_session.sh"
    "scenarios/concurrent_sessions.sh"
    "scenarios/network_stress.sh"
    "scenarios/audio_quality.sh"
)

for scenario in "${SCENARIOS[@]}"; do
    echo "🧪 Running: $scenario"
    
    START_TIME=$(date +%s)
    
    if ./$scenario >> $REPORT_FILE 2>&1; then
        STATUS="✅ PASSED"
    else
        STATUS="❌ FAILED"
    fi
    
    END_TIME=$(date +%s)
    DURATION=$((END_TIME - START_TIME))
    
    echo "" >> $REPORT_FILE
    echo "**$scenario**" >> $REPORT_FILE
    echo "- Status: $STATUS" >> $REPORT_FILE
    echo "- Duration: ${DURATION}s" >> $REPORT_FILE
    echo "" >> $REPORT_FILE
done

# Cleanup
echo "🧹 Cleaning up VMs..."
nido delete aftertalk-server --force
nido delete aftertalk-client --force
nido delete aftertalk-professional --force

echo "📊 Test report generated: $REPORT_FILE"
```

## Monitoring & Metrics

### During Test Execution

```bash
#!/bin/bash
# monitoring/collect_metrics.sh

SERVER_VM="aftertalk-server"
OUTPUT_DIR="metrics/$(date +%Y%m%d_%H%M%S)"
mkdir -p $OUTPUT_DIR

# Collect system metrics
echo "Collecting system metrics..."
nido ssh $SERVER_VM "
    # CPU and Memory
    top -b -n1 > /tmp/top.txt
    free -h > /tmp/memory.txt
    
    # Disk I/O
    iostat 1 5 > /tmp/iostat.txt
    
    # Network
    ifconfig > /tmp/network.txt
    netstat -tuln > /tmp/ports.txt
    
    # Application logs
    journalctl -u aftertalk -n 1000 > /tmp/app_logs.txt
" &

# Collect application metrics
echo "Collecting application metrics..."
curl -s http://$SERVER_IP:8080/metrics > $OUTPUT_DIR/prometheus_metrics.txt

# Collect database metrics
echo "Collecting database metrics..."
nido ssh $SERVER_VM "
    sqlite3 /opt/aftertalk/aftertalk.db '
        SELECT COUNT(*) as total_sessions FROM sessions;
        SELECT status, COUNT(*) FROM sessions GROUP BY status;
        SELECT COUNT(*) as total_transcriptions FROM transcriptions;
        SELECT COUNT(*) as total_minutes FROM minutes;
    ' > /tmp/db_stats.txt
"

# Copy metrics locally
nido ssh $SERVER_VM "tar czf - /tmp/*.txt" | tar xzf - -C $OUTPUT_DIR

echo "✅ Metrics collected in $OUTPUT_DIR"
```

### Performance Baselines

| Metric | Target | Warning | Critical |
|--------|--------|---------|----------|
| Session Creation | < 100ms | 100-500ms | > 500ms |
| Audio Latency | < 200ms | 200-500ms | > 500ms |
| Transcription | < 2s | 2-5s | > 5s |
| Minutes Generation | < 10s | 10-30s | > 30s |
| CPU Usage | < 50% | 50-80% | > 80% |
| Memory Usage | < 512MB | 512MB-1GB | > 1GB |
| Concurrent Sessions | > 100 | 50-100 | < 50 |

## CI/CD Integration

### GitHub Actions Workflow

```yaml
name: Real-World Tests

on:
  schedule:
    - cron: '0 2 * * *'  # Daily at 2 AM
  workflow_dispatch:

jobs:
  real-world-test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Install nido
        run: |
          curl -fsSL https://nido.dev/install.sh | bash
          nido doctor
      
      - name: Setup test environment
        run: |
          ./scripts/setup_test_vms.sh
      
      - name: Run test scenarios
        run: |
          ./run_all_scenarios.sh
      
      - name: Collect metrics
        run: |
          ./monitoring/collect_metrics.sh
      
      - name: Upload test report
        uses: actions/upload-artifact@v4
        with:
          name: test-report
          path: test_report_*.md
      
      - name: Cleanup
        if: always()
        run: |
          nido prune --force
```

## Troubleshooting

### VM Issues

```bash
# Check VM status
nido ls
nido info aftertalk-server

# SSH into VM for debugging
nido ssh aftertalk-server

# View VM logs
nido info aftertalk-server --logs

# Restart VM
nido stop aftertalk-server
nido start aftertalk-server
```

### Network Issues

```bash
# Test connectivity between VMs
nido ssh aftertalk-client "ping -c 3 192.168.100.2"

# Check firewall rules
nido ssh aftertalk-server "iptables -L"

# Test API from client VM
nido ssh aftertalk-client "curl http://192.168.100.2:8080/v1/health"
```

### Application Issues

```bash
# Check application logs
nido ssh aftertalk-server "journalctl -u aftertalk -f"

# Check database
nido ssh aftertalk-server "sqlite3 /opt/aftertalk/aftertalk.db '.tables'"

# Restart application
nido ssh aftertalk-server "systemctl restart aftertalk"
```

## Best Practices

### Do's

✅ Always clean up VMs after tests  
✅ Use unique session IDs for each test run  
✅ Monitor resource usage during tests  
✅ Save all logs and metrics  
✅ Run tests in isolated networks  
✅ Document any failures or issues  

### Don'ts

❌ Don't run tests on production infrastructure  
❌ Don't share API keys or credentials  
❌ Don't leave VMs running overnight  
❌ Don't ignore resource warnings  
❌ Don't run concurrent tests without isolation  

---

## Resources

- [Nido Documentation](https://nido.dev/docs)
- [Aftertalk API Documentation](../specs/contracts/api.yaml)
- [Testing Strategy](testing.md)

---

Last updated: 2025-03-04
