/**
 * WebSocket Client with Automatic Reconnection
 * Implements Requirements 9.1, 9.4 - WebSocket connection with automatic reconnection
 */
class WebSocketClient {
    constructor(url, options = {}) {
        this.url = url;
        this.options = {
            maxReconnectAttempts: options.maxReconnectAttempts || 10,
            reconnectInterval: options.reconnectInterval || 1000, // Start with 1 second
            maxReconnectInterval: options.maxReconnectInterval || 30000, // Max 30 seconds
            reconnectDecay: options.reconnectDecay || 1.5, // Exponential backoff multiplier
            timeoutInterval: options.timeoutInterval || 2000,
            enableLogging: options.enableLogging || false,
            ...options
        };

        this.ws = null;
        this.reconnectAttempts = 0;
        this.reconnectTimeoutId = null;
        this.isReconnecting = false;
        this.forcedClose = false;
        this.timedOut = false;

        // Event handlers
        this.onopen = null;
        this.onclose = null;
        this.onmessage = null;
        this.onerror = null;
        this.onreconnect = null;
        this.onmaxreconnect = null;

        // Bind methods to preserve context
        this.open = this.open.bind(this);
        this.close = this.close.bind(this);
        this.send = this.send.bind(this);
        this.reconnect = this.reconnect.bind(this);
    }

    /**
     * Open WebSocket connection
     * Requirement 9.1: Establish WebSocket connections
     */
    open(reconnectAttempt = false) {
        this.ws = new WebSocket(this.url);

        if (this.options.enableLogging) {
            console.log('WebSocketClient: Attempting to connect to', this.url);
        }

        const localWs = this.ws;
        const timeout = setTimeout(() => {
            if (this.options.enableLogging) {
                console.log('WebSocketClient: Connection timeout');
            }
            this.timedOut = true;
            localWs.close();
            this.timedOut = false;
        }, this.options.timeoutInterval);

        this.ws.onopen = (event) => {
            clearTimeout(timeout);
            if (this.options.enableLogging) {
                console.log('WebSocketClient: Connected');
            }

            this.reconnectAttempts = 0;
            this.isReconnecting = false;

            if (reconnectAttempt && this.onreconnect) {
                this.onreconnect(event);
            }

            if (this.onopen) {
                this.onopen(event);
            }
        };

        this.ws.onclose = (event) => {
            clearTimeout(timeout);
            this.ws = null;

            if (this.forcedClose) {
                if (this.options.enableLogging) {
                    console.log('WebSocketClient: Connection closed (forced)');
                }
                if (this.onclose) {
                    this.onclose(event);
                }
                return;
            }

            if (this.options.enableLogging) {
                console.log('WebSocketClient: Connection closed', event.code, event.reason);
            }

            // Attempt to reconnect unless max attempts reached
            // Requirement 9.4: Automatic reconnection with exponential backoff
            if (this.reconnectAttempts < this.options.maxReconnectAttempts) {
                this.reconnect();
            } else {
                if (this.options.enableLogging) {
                    console.log('WebSocketClient: Max reconnection attempts reached');
                }
                if (this.onmaxreconnect) {
                    this.onmaxreconnect(event);
                }
            }

            if (this.onclose) {
                this.onclose(event);
            }
        };

        this.ws.onmessage = (event) => {
            if (this.onmessage) {
                this.onmessage(event);
            }
        };

        this.ws.onerror = (event) => {
            if (this.options.enableLogging) {
                console.log('WebSocketClient: Error occurred');
            }
            if (this.onerror) {
                this.onerror(event);
            }
        };
    }

    /**
     * Send message through WebSocket
     */
    send(data) {
        if (this.ws && this.ws.readyState === WebSocket.OPEN) {
            this.ws.send(data);
            return true;
        } else {
            if (this.options.enableLogging) {
                console.log('WebSocketClient: Cannot send message, connection not open');
            }
            return false;
        }
    }

    /**
     * Close WebSocket connection
     */
    close(code = 1000, reason = '') {
        this.forcedClose = true;
        if (this.reconnectTimeoutId) {
            clearTimeout(this.reconnectTimeoutId);
            this.reconnectTimeoutId = null;
        }
        if (this.ws) {
            this.ws.close(code, reason);
        }
    }

    /**
     * Reconnect with exponential backoff
     * Requirement 9.4: Exponential backoff for reconnection attempts
     */
    reconnect() {
        if (this.isReconnecting || this.forcedClose) {
            return;
        }

        this.isReconnecting = true;
        this.reconnectAttempts++;

        // Calculate delay with exponential backoff
        const delay = Math.min(
            this.options.reconnectInterval * Math.pow(this.options.reconnectDecay, this.reconnectAttempts - 1),
            this.options.maxReconnectInterval
        );

        if (this.options.enableLogging) {
            console.log(`WebSocketClient: Reconnecting in ${delay}ms (attempt ${this.reconnectAttempts}/${this.options.maxReconnectAttempts})`);
        }

        this.reconnectTimeoutId = setTimeout(() => {
            this.reconnectTimeoutId = null;
            this.open(true);
        }, delay);
    }

    /**
     * Get current connection state
     */
    getReadyState() {
        if (this.ws) {
            return this.ws.readyState;
        }
        return WebSocket.CLOSED;
    }

    /**
     * Check if connection is open
     */
    isConnected() {
        return this.ws && this.ws.readyState === WebSocket.OPEN;
    }

    /**
     * Get connection statistics
     */
    getStats() {
        return {
            reconnectAttempts: this.reconnectAttempts,
            isReconnecting: this.isReconnecting,
            isConnected: this.isConnected(),
            readyState: this.getReadyState()
        };
    }
}

/**
 * Uptime Monitor WebSocket Client
 * Handles real-time updates for the uptime monitoring dashboard
 */
class UptimeMonitorWebSocket {
    constructor(options = {}) {
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const host = options.host || window.location.host;
        const path = options.path || '/ws';
        
        this.url = `${protocol}//${host}${path}`;
        this.options = {
            enableLogging: options.enableLogging || false,
            maxReconnectAttempts: options.maxReconnectAttempts || 10,
            ...options
        };

        this.client = new WebSocketClient(this.url, this.options);
        this.messageHandlers = new Map();
        this.connectionListeners = [];

        this.setupEventHandlers();
    }

    /**
     * Setup WebSocket event handlers
     */
    setupEventHandlers() {
        this.client.onopen = (event) => {
            if (this.options.enableLogging) {
                console.log('UptimeMonitor: Connected to WebSocket');
            }
            this.notifyConnectionListeners('connected', event);
        };

        this.client.onclose = (event) => {
            if (this.options.enableLogging) {
                console.log('UptimeMonitor: Disconnected from WebSocket');
            }
            this.notifyConnectionListeners('disconnected', event);
        };

        this.client.onreconnect = (event) => {
            if (this.options.enableLogging) {
                console.log('UptimeMonitor: Reconnected to WebSocket');
            }
            this.notifyConnectionListeners('reconnected', event);
        };

        this.client.onmaxreconnect = (event) => {
            if (this.options.enableLogging) {
                console.log('UptimeMonitor: Max reconnection attempts reached');
            }
            this.notifyConnectionListeners('maxreconnect', event);
        };

        this.client.onerror = (event) => {
            if (this.options.enableLogging) {
                console.log('UptimeMonitor: WebSocket error');
            }
            this.notifyConnectionListeners('error', event);
        };

        this.client.onmessage = (event) => {
            try {
                const message = JSON.parse(event.data);
                this.handleMessage(message);
            } catch (error) {
                if (this.options.enableLogging) {
                    console.error('UptimeMonitor: Failed to parse message:', error);
                }
            }
        };
    }

    /**
     * Handle incoming WebSocket messages
     */
    handleMessage(message) {
        const { type, data } = message;
        
        if (this.messageHandlers.has(type)) {
            const handlers = this.messageHandlers.get(type);
            handlers.forEach(handler => {
                try {
                    handler(data);
                } catch (error) {
                    if (this.options.enableLogging) {
                        console.error(`UptimeMonitor: Error in ${type} handler:`, error);
                    }
                }
            });
        }

        if (this.options.enableLogging) {
            console.log(`UptimeMonitor: Received ${type} message:`, data);
        }
    }

    /**
     * Add message handler for specific message type
     */
    on(messageType, handler) {
        if (!this.messageHandlers.has(messageType)) {
            this.messageHandlers.set(messageType, []);
        }
        this.messageHandlers.get(messageType).push(handler);
    }

    /**
     * Remove message handler
     */
    off(messageType, handler) {
        if (this.messageHandlers.has(messageType)) {
            const handlers = this.messageHandlers.get(messageType);
            const index = handlers.indexOf(handler);
            if (index > -1) {
                handlers.splice(index, 1);
            }
        }
    }

    /**
     * Add connection state listener
     */
    onConnection(listener) {
        this.connectionListeners.push(listener);
    }

    /**
     * Notify connection listeners
     */
    notifyConnectionListeners(state, event) {
        this.connectionListeners.forEach(listener => {
            try {
                listener(state, event);
            } catch (error) {
                if (this.options.enableLogging) {
                    console.error('UptimeMonitor: Error in connection listener:', error);
                }
            }
        });
    }

    /**
     * Connect to WebSocket
     */
    connect() {
        this.client.open();
    }

    /**
     * Disconnect from WebSocket
     */
    disconnect() {
        this.client.close();
    }

    /**
     * Get connection status
     */
    isConnected() {
        return this.client.isConnected();
    }

    /**
     * Get connection statistics
     */
    getStats() {
        return this.client.getStats();
    }
}

// Export for use in modules or make available globally
if (typeof module !== 'undefined' && module.exports) {
    module.exports = { WebSocketClient, UptimeMonitorWebSocket };
} else {
    window.WebSocketClient = WebSocketClient;
    window.UptimeMonitorWebSocket = UptimeMonitorWebSocket;
}