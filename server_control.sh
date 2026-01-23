#!/bin/bash
set -uo pipefail

APP_NAME="ledger-server"
LOG_DIR_DEFAULT="logs"
LOG_FILE_DEFAULT="server.log"
PID_FILE="server.pid"

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

function check_pid() {
    if [ -f "$PID_FILE" ]; then
        PID="$(cat "$PID_FILE" 2>/dev/null || true)"
        if [ -n "$PID" ] && [[ "$PID" =~ ^[0-9]+$ ]]; then
            if kill -0 "$PID" > /dev/null 2>&1; then
                return 0
            fi
        fi
    fi
    return 1
}

function start() {
    echo "Checking if $APP_NAME is already running..."
    if check_pid; then
        echo -e "${GREEN}$APP_NAME is already running (PID: $(cat "$PID_FILE")).${NC}"
        exit 0
    fi

    if [ -f "$PID_FILE" ]; then
        echo -e "${YELLOW}Stale PID file found ($PID_FILE), cleaning up...${NC}"
        rm -f "$PID_FILE"
    fi

    echo "Starting $APP_NAME..."

    if [ ! -f "$APP_NAME" ]; then
        echo -e "${RED}Error: $APP_NAME not found. Run ./build.sh first.${NC}"
        exit 1
    fi

    if [ ! -x "$APP_NAME" ]; then
        echo "Making $APP_NAME executable..."
        chmod +x "$APP_NAME"
    fi

    : "${PORT:=8080}"
    export PORT

    : "${LOG_DIR:=$LOG_DIR_DEFAULT}"
    export LOG_DIR
    mkdir -p "$LOG_DIR"

    : "${LOG_FILE:=$LOG_FILE_DEFAULT}"
    export LOG_FILE

    export PID_FILE="$PID_FILE"

    echo "Starting $APP_NAME on port $PORT..."
    echo "Logs will be written to $LOG_DIR/$LOG_FILE"

    nohup ./"$APP_NAME" >"$LOG_DIR/$LOG_FILE" 2>&1 &
    PID=$!
    echo $PID > "$PID_FILE"
    echo "Process started with PID: $PID"

    echo "Waiting for process to stabilize..."
    sleep 2

    if check_pid; then
        echo -e "${GREEN}$APP_NAME started successfully (PID: $(cat "$PID_FILE")).${NC}"
        echo "Logs are being written to $LOG_DIR/$LOG_FILE"
        echo "Access at http://localhost:$PORT"
    else
        echo -e "${RED}Failed to start $APP_NAME.${NC}"
        echo -e "${RED}Check $LOG_DIR/$LOG_FILE for details:${NC}"
        if [ -f "$LOG_DIR/$LOG_FILE" ]; then
            tail -30 "$LOG_DIR/$LOG_FILE"
        fi
        rm -f "$PID_FILE"
        exit 1
    fi
}

function stop() {
    if check_pid; then
        PID=$(cat "$PID_FILE")
        echo "Stopping $APP_NAME (PID: $PID)..."
        kill "$PID" 2>/dev/null || true

        for i in {1..10}; do
            if ! check_pid; then
                break
            fi
            echo "Waiting for process to stop... ($i/10)"
            sleep 1
        done

        if check_pid; then
            echo "Force killing process..."
            kill -9 "$PID" 2>/dev/null || true
            sleep 1
        fi

        rm -f "$PID_FILE"
        echo -e "${GREEN}$APP_NAME stopped.${NC}"
    else
        if [ -f "$PID_FILE" ]; then
            echo -e "${YELLOW}Stale PID file found ($PID_FILE), but process is not running. Cleaning up...${NC}"
            rm -f "$PID_FILE"
        fi
        echo -e "${RED}$APP_NAME is not running.${NC}"
    fi
}

function restart() {
    stop
    sleep 1
    start
}

function status() {
    if check_pid; then
        echo -e "${GREEN}$APP_NAME is running (PID: $(cat "$PID_FILE")).${NC}"
    else
        if [ -f "$PID_FILE" ]; then
            echo -e "${YELLOW}Stale PID file found ($PID_FILE), but process is not running.${NC}"
        fi
        echo -e "${RED}$APP_NAME is not running.${NC}"
    fi
}

case "${1:-}" in
    start)
        start
        ;;
    stop)
        stop
        ;;
    restart)
        restart
        ;;
    status)
        status
        ;;
    *)
        echo "Usage: $0 {start|stop|restart|status}"
        exit 1
        ;;
esac
