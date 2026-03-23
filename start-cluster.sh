#!/usr/bin/env bash

set -euo pipefail

usage() {
    echo "Usage: $0 <node-count> [--eviction <FIFO|LRU|NONE>] [--storage <bytes>]"
    echo "       $0 <node-count> [-e <FIFO|LRU|NONE>] [-s <bytes>]"
    echo "Examples:"
    echo "  $0 3"
    echo "  $0 5 --eviction LRU --storage 1048576"
}

NODE_COUNT=""
EVICTION=""
STORAGE_SIZE=""

while [[ $# -gt 0 ]]; do
    case "$1" in
        -e|--eviction)
            if [[ $# -lt 2 ]]; then
                echo "Error: Missing value for $1"
                usage
                exit 1
            fi
            EVICTION="$2"
            shift 2
            ;;
        -s|--storage)
            if [[ $# -lt 2 ]]; then
                echo "Error: Missing value for $1"
                usage
                exit 1
            fi
            STORAGE_SIZE="$2"
            shift 2
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        -* )
            echo "Error: Unknown option $1"
            usage
            exit 1
            ;;
        *)
            if [[ -z "$NODE_COUNT" ]]; then
                NODE_COUNT="$1"
                shift
            else
                echo "Error: Unexpected argument '$1'"
                usage
                exit 1
            fi
            ;;
    esac
done

if [[ -z "$NODE_COUNT" ]]; then
    echo "Error: <node-count> is required"
    usage
    exit 1
fi

if ! [[ "$NODE_COUNT" =~ ^[0-9]+$ ]] || [[ "$NODE_COUNT" -lt 1 ]]; then
    echo "Error: <node-count> must be a positive integer"
    usage
    exit 1
fi

if [[ -n "$EVICTION" ]]; then
    case "$EVICTION" in
        FIFO|LRU|NONE)
            ;;
        *)
            echo "Error: --eviction/-e must be one of FIFO, LRU, NONE"
            exit 1
            ;;
    esac
fi

if [[ -n "$STORAGE_SIZE" ]]; then
    if ! [[ "$STORAGE_SIZE" =~ ^[0-9]+$ ]] || [[ "$STORAGE_SIZE" -lt 1 ]]; then
        echo "Error: --storage/-s must be a positive integer (bytes)"
        exit 1
    fi
fi

BASE_HTTP_PORT=8090
BASE_GRPC_PORT=9090
BASE_CLUSTER_PORT=7990
JOIN_TIMEOUT_SECONDS=60

mkdir -p ./tmp

declare -a PIDS=()
declare -a HTTP_PORTS=()
declare -a QAD_EXTRA_ARGS=()

if [[ -n "$EVICTION" ]]; then
    QAD_EXTRA_ARGS+=("-eviction" "$EVICTION")
fi

if [[ -n "$STORAGE_SIZE" ]]; then
    QAD_EXTRA_ARGS+=("-storage-size" "$STORAGE_SIZE")
fi

cleanup() {
    trap - EXIT INT TERM

    if [[ "${#PIDS[@]}" -gt 0 ]]; then
        echo ""
        echo "Stopping ${#PIDS[@]} node(s)..."
        for pid in "${PIDS[@]}"; do
            kill "$pid" 2>/dev/null || true
        done
        wait "${PIDS[@]}" 2>/dev/null || true
    fi
}

trap cleanup EXIT INT TERM

start_node() {
    local idx="$1"
    local http_port="$((BASE_HTTP_PORT + idx))"
    local grpc_port="$((BASE_GRPC_PORT + idx))"
    local cluster_port="$((BASE_CLUSTER_PORT + idx))"
    local node_name
    node_name="node-$(printf "%02d" "$((idx + 1))")"
    local log_file
    log_file="./tmp/${node_name}.log"

    HTTP_PORTS+=("$http_port")

    if [[ "$idx" -eq 0 ]]; then
        NODE_NAME="$node_name" ./qad \
            -port "$http_port" \
            -grpc-port "$grpc_port" \
            -cluster-port "$cluster_port" \
            "${QAD_EXTRA_ARGS[@]}" \
            > "$log_file" 2>&1 &
    else
        local seed_addr
        seed_addr="127.0.0.1:${BASE_CLUSTER_PORT}"
        NODE_NAME="$node_name" SEED_NODES="$seed_addr" ./qad \
            -port "$http_port" \
            -grpc-port "$grpc_port" \
            -cluster-port "$cluster_port" \
            "${QAD_EXTRA_ARGS[@]}" \
            > "$log_file" 2>&1 &
    fi

    local pid=$!
    PIDS+=("$pid")

    echo "Started ${node_name} (PID=${pid})"
    echo "  - HTTP:    ${http_port}"
    echo "  - gRPC:    ${grpc_port}"
    echo "  - Cluster: ${cluster_port}"
    echo "  - Logs:    ${log_file}"
}

wait_for_cluster_convergence_on_node() {
    local http_port="$1"
    local expected_nodes="$2"

    local deadline=$((SECONDS + JOIN_TIMEOUT_SECONDS))
    while (( SECONDS < deadline )); do
        local response
        response="$(curl -fsS "http://127.0.0.1:${http_port}/cluster" 2>/dev/null || true)"

        if [[ -n "$response" ]]; then
            local total_nodes
            total_nodes="$(echo "$response" | sed -n 's/.*"total_nodes"[[:space:]]*:[[:space:]]*\([0-9][0-9]*\).*/\1/p')"

            if [[ -n "$total_nodes" && "$total_nodes" -eq "$expected_nodes" ]]; then
                echo "Node on port ${http_port} reports total_nodes=${total_nodes}"
                return 0
            fi
        fi

        sleep 1
    done

    echo "Timed out waiting for node on port ${http_port} to report total_nodes=${expected_nodes}"
    return 1
}

if [[ ! -x ./qad ]]; then
    echo "Error: ./qad binary not found or not executable. Build it first (e.g. make build)."
    exit 1
fi

echo "Starting ${NODE_COUNT} node(s)..."
for ((i = 0; i < NODE_COUNT; i++)); do
    start_node "$i"
done

echo ""
echo "Waiting for cluster convergence (expected total_nodes=${NODE_COUNT})..."
for port in "${HTTP_PORTS[@]}"; do
    wait_for_cluster_convergence_on_node "$port" "$NODE_COUNT"
done

echo ""
echo "Cluster is healthy. Press Ctrl+C to stop all nodes."

wait