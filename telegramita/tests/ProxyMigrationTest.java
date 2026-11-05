import android.content.SharedPreferences;
import java.lang.reflect.Method;
import java.lang.reflect.Proxy;
import java.util.HashMap;
import java.util.Map;
import org.telegram.messenger.SharedConfig.ProxyInfo;
import org.telegram.tgnet.InputSerializedData;
import org.telegram.tgnet.OutputSerializedData;
import org.telegram.tgnet.SerializedData;
import org.telegram.utils.proxy.ProxySettings;

/** Runs actual compiled Telegramita model/codec on the host, without a device. */
public final class ProxyMigrationTest {
    private static final Method READ;
    private static final Method WRITE;
    static {
        try {
            READ = ProxyInfo.class.getDeclaredMethod("fromSerializedData", int.class, InputSerializedData.class);
            WRITE = ProxyInfo.class.getDeclaredMethod("toSerializedData", OutputSerializedData.class);
            READ.setAccessible(true);
            WRITE.setAccessible(true);
        } catch (Exception e) { throw new RuntimeException(e); }
    }
    private static void check(boolean condition, String message) {
        if (!condition) { throw new AssertionError(message); }
    }
    private static ProxyInfo read(int version, SerializedData data) throws Exception {
        byte[] bytes = data.toByteArray();
        SerializedData input = new SerializedData(bytes);
        ProxyInfo info = (ProxyInfo) READ.invoke(null, version, input);
        check(input.remaining() == 0, "Codec did not consume complete record v" + version);
        return info;
    }
    private static SerializedData legacy(int version, String protocol, String ports) {
        SerializedData data = new SerializedData();
        data.writeString("example.com");
        data.writeInt32(7070);
        data.writeString("fixture-user");
        data.writeString("fixture-password");
        data.writeString("");
        if (version >= 3) {
            data.writeInt32(2);
            data.writeInt32(1380);
            data.writeString(protocol);
            if (version == 4) { data.writeString(ports); }
        }
        data.writeInt64(123);
        data.writeInt64(456);
        return data;
    }
    private static void validate(ProxySettings settings, String protocol, String ports) {
        check(settings.getType() == ProxySettings.Type.MIERU, "Legacy type 2 must remain Mieru");
        check(settings.getAddress().equals("example.com"), "Server lost");
        check(settings.getUser().equals("fixture-user"), "User lost");
        check(settings.getPassword().equals("fixture-password"), "Password lost");
        check(settings.getMieruMtu() == 1380, "MTU lost");
        check(settings.getMieruProtocol().equals(protocol), "Transport lost");
        check(settings.getMieruPort().equals(ports), "Port/range lost");
        check(settings.isValid(), "Migrated configuration invalid");
    }
    private static void preferences(ProxySettings settings) {
        Map<String, Object> values = new HashMap<>();
        SharedPreferences.Editor editor = (SharedPreferences.Editor) Proxy.newProxyInstance(
                ProxyMigrationTest.class.getClassLoader(), new Class<?>[] { SharedPreferences.Editor.class },
                (p, method, args) -> {
                    if (method.getName().startsWith("put")) { values.put((String) args[0], args[1]); return p; }
                    if (method.getName().equals("remove")) { values.remove(args[0]); return p; }
                    return null;
                });
        settings.toSharedPreferences(editor);
        SharedPreferences prefs = (SharedPreferences) Proxy.newProxyInstance(
                ProxyMigrationTest.class.getClassLoader(), new Class<?>[] { SharedPreferences.class },
                (p, method, args) -> values.getOrDefault((String) args[0], args[1]));
        check(settings.equals(ProxySettings.fromSharedPreferences(prefs)), "Active preferences round-trip failed");
        check(values.get("proxy_type").equals(2), "Stored Mieru type changed");
    }
    public static void main(String[] args) throws Exception {
        int cases = 0;
        for (String protocol : new String[] { "TCP", "UDP", "TCP+UDP" }) {
            for (int version : new int[] { 3, 4 }) {
                String ports = version == 3 ? "7070" : "7070-7079";
                ProxyInfo migrated = read(version, legacy(version, protocol, ports));
                validate(migrated.settings, protocol, ports);
                check(migrated.ping == 123 && migrated.availableCheckTime == 456, "Metadata lost");
                preferences(migrated.settings);
                SerializedData encoded = new SerializedData();
                WRITE.invoke(migrated, encoded);
                ProxyInfo restored = read(5, encoded);
                validate(restored.settings, protocol, ports);
                check(restored.settings.equals(migrated.settings), "V5 round-trip failed");
                cases++;
            }
        }
        ProxyInfo socks = read(2, legacy(2, "TCP", "7070"));
        check(socks.settings.getType() == ProxySettings.Type.SOCKS5, "V2 SOCKS migration failed");
        check(ProxySettings.typeToInt(ProxySettings.Type.WEB) == 3, "WEB and legacy Mieru types collide");
        System.out.println("PASS: " + cases + " legacy Mieru migration/persistence cases, V2 SOCKS, WEB type separation");
    }
}
