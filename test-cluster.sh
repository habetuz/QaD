$ # Clean up
pkill -f "qad -port" 2>/dev/null || true
sleep 2
echo "=== Final Cross-Node Test ==="
./qad -port 8090 -grpc-port 9090 -cluster-port 7990 > /tmp/final-node1.log 2>&1 &
N1=$!
echo "Started Node 1 (PID=$N1)"
sleep 3
SEED_NODES="127.0.0.1:7990" ./qad -port 8091 -grpc-port 9091 -cluster-port 7991 > /tmp/final-node2.log 2>&1 &
N2=$!
echo "Started Node 2 (PID=$N2)"
sleep 4
echo ""
echo "Test 1: Write to Node 1, read from Node 2"
curl -s -X POST http://localhost:8090/apple -d "red fruit" > /dev/null
sleep 1
RESULT=$(curl -s http://localhost:8091/apple)
if [ "$RESULT" = "red fruit" ]; then
    echo "✅ PASS: $RESULT"
else
    echo "❌ FAIL: Expected 'red fruit', got '$RESULT'"
fi
echo ""
echo "Test 2: Write to Node 2, read from Node 1"
curl -s -X POST http://localhost:8091/banana -d "yellow fruit" > /dev/null
sleep 1
RESULT=$(curl -s http://localhost:8090/banana)
if [ "$RESULT" = "yellow fruit" ]; then
    echo "✅ PASS: $RESULT"
else
    echo "❌ FAIL: Expected 'yellow fruit', got '$RESULT'"
fi
echo ""
echo "Test 3: Write to Node 1, read from Node 1"
curl -s -X POST http://localhost:8090/cherry -d "small fruit" > /dev/null
sleep 1
RESULT=$(curl -s http://localhost:8090/cherry)
if [ "$RESULT" = "small fruit" ]; then
    echo "✅ PASS: $RESULT"
else
    echo "❌ FAIL: Expected 'small fruit', got '$RESULT'"
fi
echo ""
echo "Test 4: Write to Node 2, read from Node 2"
curl -s -X POST http://localhost:8091/date -d "sweet fruit" > /dev/null
sleep 1
RESULT=$(curl -s http://localhost:8091/date)
if [ "$RESULT" = "sweet fruit" ]; then
    echo "✅ PASS: $RESULT"
else
    echo "❌ FAIL: Expected 'sweet fruit', got '$RESULT'"
fi
echo ""
echo "Test 5: Non-existent key"
RESULT=$(curl -s http://localhost:8090/nonexistent)
if [ "$RESULT" = "Key not found" ]; then
    echo "✅ PASS: Key not found as expected"
else
    echo "❌ FAIL: Expected 'Key not found', got '$RESULT'"
fi
kill $N1 $N2 2>/dev/null || true
echo ""
echo "All tests complete!"