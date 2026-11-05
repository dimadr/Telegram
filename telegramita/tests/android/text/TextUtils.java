package android.text;
/** Minimal host shim; production Android provides this method. */
public final class TextUtils {
    public static boolean isEmpty(CharSequence value) { return value == null || value.length() == 0; }
}
