package org.telegram.messenger;

import android.util.Log;

import org.json.JSONObject;

/**
 * Native Mieru proxy client wrapper.
 * Provides interface to the Mieru proxy library compiled via cgo c-shared.
 */
public class MieruClient {

    private static final String TAG = "MieruClient";

    // Native client pointer
    private long nativeClientPtr;

    // Singleton instance
    private static MieruClient instance;

    // State
    private boolean configured = false;
    private boolean running = false;
    private ProxyConfig currentConfig;

    /**
     * Proxy configuration for Mieru.
     */
    public static class ProxyConfig {
        public String serverAddress;
        public String portSpec;
        public String username;
        public String password;
        public int mtu;
        public String protocol; // "TCP" or "UDP"

        public ProxyConfig() {
            this.mtu = 1400;
            this.protocol = "TCP";
        }

        public ProxyConfig(String serverAddress, String portSpec, String username, String password) {
            this();
            this.serverAddress = serverAddress;
            this.portSpec = portSpec;
            this.username = username;
            this.password = password;
        }
    }

    private MieruClient() {
        nativeClientPtr = nativeNewClient();
        Log.d(TAG, "MieruClient created");
    }

    /**
     * Get singleton instance of MieruClient.
     */
    public static synchronized MieruClient getInstance() {
        if (instance == null) {
            instance = new MieruClient();
        }
        return instance;
    }

    public static synchronized void stopIfCreated() {
        if (instance != null) {
            instance.stop();
        }
    }

    private synchronized void ensureClient() {
        if (nativeClientPtr == 0) {
            nativeClientPtr = nativeNewClient();
        }
    }

    private synchronized void destroyClient() {
        if (nativeClientPtr != 0) {
            nativeFreeClient(nativeClientPtr);
            nativeClientPtr = 0;
        }
        configured = false;
        running = false;
    }

    /**
     * Configure the Mieru client with proxy settings.
     */
    public synchronized boolean configure(ProxyConfig config) {
        if (config == null) {
            return false;
        }
        if (running) {
            Log.w(TAG, "Client is running, stopping first");
            stop();
        }
        ensureClient();
        if (nativeClientPtr == 0) {
            Log.e(TAG, "Client not initialized");
            return false;
        }

        currentConfig = config;
        boolean result = nativeConfigure(
                nativeClientPtr,
                config.serverAddress,
                config.portSpec,
                config.username,
                config.password,
                config.mtu,
                config.protocol
        );

        configured = result;
        if (result) {
            Log.d(TAG, "Client configured: " + config.serverAddress + ":" + config.portSpec);
        } else {
            Log.e(TAG, "Failed to configure client");
        }

        return result;
    }

    /**
     * Start the Mieru client.
     */
    public synchronized boolean start() {
        if (nativeClientPtr == 0) {
            Log.e(TAG, "Client not initialized");
            return false;
        }

        if (!configured) {
            Log.e(TAG, "Client not configured");
            return false;
        }

        if (running) {
            Log.w(TAG, "Client already running");
            return true;
        }

        boolean started = nativeStart(nativeClientPtr);
        if (started) {
            nativeActivate(nativeClientPtr);
            running = true;
            Log.d(TAG, "Client started");
        } else {
            Log.e(TAG, "Failed to start client");
        }

        return started;
    }

    /**
     * Stop the Mieru client. The underlying Go client can't be reused after
     * stop, so the handle is released and recreated on the next configure.
     */
    public synchronized void stop() {
        if (nativeClientPtr != 0 && running) {
            nativeDeactivate();
            nativeStop(nativeClientPtr);
        }
        destroyClient();
        Log.d(TAG, "Client stopped");
    }

    /**
     * Check if client is running.
     */
    public synchronized boolean isRunning() {
        if (nativeClientPtr == 0) {
            return false;
        }
        return nativeIsRunning(nativeClientPtr);
    }

    /**
     * Get current configuration.
     */
    public ProxyConfig getCurrentConfig() {
        return currentConfig;
    }

    /**
     * Parse a Mieru URL (mierus:// or mieru://).
     */
    public static ProxyConfig parseURL(String url) {
        if (url == null || url.isEmpty()) {
            return null;
        }

        try {
            String parsed = nativeParseURL(url);
            if (parsed == null) {
                return null;
            }
            JSONObject json = new JSONObject(parsed);
            ProxyConfig config = new ProxyConfig(
                    json.getString("server"),
                    json.getString("port"),
                    json.getString("username"),
                    json.getString("password")
            );
            config.mtu = json.getInt("mtu");
            config.protocol = json.getString("protocol");
            return config;
        } catch (Exception e) {
            Log.e(TAG, "Failed to parse URL: " + e.getMessage());
            return null;
        }
    }

    /**
     * Create a Mieru URL from configuration.
     */
    public static String createURL(ProxyConfig config) {
        if (config == null) {
            return null;
        }

        return nativeCreateURL(config.serverAddress, config.portSpec, config.username,
                config.password, config.mtu, config.protocol);
    }

    public static boolean isValidPortSpec(String portSpec) {
        if (portSpec == null || portSpec.isEmpty()) {
            return false;
        }
        String[] parts = portSpec.split("-", -1);
        if (parts.length < 1 || parts.length > 2) {
            return false;
        }
        try {
            int first = Integer.parseInt(parts[0]);
            int last = parts.length == 2 ? Integer.parseInt(parts[1]) : first;
            return first >= 1 && last <= 65535 && first <= last;
        } catch (NumberFormatException e) {
            return false;
        }
    }

    /**
     * Cleanup resources.
     */
    public synchronized void destroy() {
        stop();
        instance = null;
        Log.d(TAG, "Client destroyed");
    }

    // Native methods
    private native long nativeNewClient();
    private native boolean nativeConfigure(long clientPtr, String serverAddress, String portSpec,
                                           String username, String password, int mtu, String protocol);
    private native boolean nativeStart(long clientPtr);
    private native void nativeStop(long clientPtr);
    private native boolean nativeIsRunning(long clientPtr);
    private native void nativeActivate(long clientPtr);
    private native void nativeDeactivate();
    private native void nativeFreeClient(long clientPtr);
    private static native String nativeParseURL(String url);
    private static native String nativeCreateURL(String serverAddress, String portSpec,
                                                 String username, String password, int mtu, String protocol);
}
