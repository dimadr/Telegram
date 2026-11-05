// JNI bridge for Mieru proxy client.
//
// Provides the JNI interface between Java (org.telegram.messenger.MieruClient)
// and the Go c-shared library (libmieru.so). Dial completion callbacks are
// registered by tgnet (TgNetWrapper.cpp) directly into the Go library as
// function pointers, so there is no callback layer here.
//
// The Go library is built by the mieru-android module using Go cgo with
// -buildmode=c-shared. See _research/mieru-android/build.sh for details.

#include <jni.h>
#include <stdlib.h>
#include <string.h>

#include <android/log.h>
#define LOGI(...) __android_log_print(ANDROID_LOG_INFO, "MieruJNI", __VA_ARGS__)
#define LOGE(...) __android_log_print(ANDROID_LOG_ERROR, "MieruJNI", __VA_ARGS__)

#include "mieru_jni.h"

// ---------------------------------------------------------------------------
// Java class: org.telegram.messenger.MieruClient
// ---------------------------------------------------------------------------

JNIEXPORT jlong JNICALL
Java_org_telegram_messenger_MieruClient_nativeNewClient(JNIEnv *env, jobject thiz) {
    int64_t handle = mieruclient_new();
    LOGI("nativeNewClient -> %lld", (long long) handle);
    return (jlong) handle;
}

JNIEXPORT jboolean JNICALL
Java_org_telegram_messenger_MieruClient_nativeConfigure(
    JNIEnv *env, jobject thiz, jlong handle,
    jstring serverAddress, jstring portSpec,
    jstring username, jstring password,
    jint mtu, jstring protocol) {

    if (handle == 0) {
        LOGE("nativeConfigure: invalid handle");
        return JNI_FALSE;
    }

    const char *addrStr = serverAddress ? (*env)->GetStringUTFChars(env, serverAddress, NULL) : NULL;
    const char *portStr = portSpec ? (*env)->GetStringUTFChars(env, portSpec, NULL) : NULL;
    const char *userStr = username ? (*env)->GetStringUTFChars(env, username, NULL) : NULL;
    const char *passStr = password ? (*env)->GetStringUTFChars(env, password, NULL) : NULL;
    const char *protoStr = protocol ? (*env)->GetStringUTFChars(env, protocol, NULL) : NULL;

    int result = mieruclient_configure((int64_t) handle, addrStr, portStr, userStr, passStr, (int32_t) mtu, protoStr);

    if (addrStr) (*env)->ReleaseStringUTFChars(env, serverAddress, addrStr);
    if (portStr) (*env)->ReleaseStringUTFChars(env, portSpec, portStr);
    if (userStr) (*env)->ReleaseStringUTFChars(env, username, userStr);
    if (passStr) (*env)->ReleaseStringUTFChars(env, password, passStr);
    if (protoStr) (*env)->ReleaseStringUTFChars(env, protocol, protoStr);

    return result ? JNI_TRUE : JNI_FALSE;
}

JNIEXPORT jboolean JNICALL
Java_org_telegram_messenger_MieruClient_nativeStart(JNIEnv *env, jobject thiz, jlong handle) {
    if (handle == 0) {
        return JNI_FALSE;
    }
    int started = mieruclient_start((int64_t) handle);
    LOGI("nativeStart -> %d", started);
    return started ? JNI_TRUE : JNI_FALSE;
}

JNIEXPORT void JNICALL
Java_org_telegram_messenger_MieruClient_nativeStop(JNIEnv *env, jobject thiz, jlong handle) {
    if (handle == 0) {
        return;
    }
    mieruclient_stop((int64_t) handle);
}

JNIEXPORT jboolean JNICALL
Java_org_telegram_messenger_MieruClient_nativeIsRunning(JNIEnv *env, jobject thiz, jlong handle) {
    if (handle == 0) {
        return JNI_FALSE;
    }
    return mieruclient_is_running((int64_t) handle) ? JNI_TRUE : JNI_FALSE;
}

JNIEXPORT void JNICALL
Java_org_telegram_messenger_MieruClient_nativeActivate(JNIEnv *env, jobject thiz, jlong handle) {
    if (handle == 0) {
        return;
    }
    mieruclient_activate((int64_t) handle);
}

JNIEXPORT void JNICALL
Java_org_telegram_messenger_MieruClient_nativeDeactivate(JNIEnv *env, jobject thiz) {
    mieruclient_deactivate();
}

JNIEXPORT void JNICALL
Java_org_telegram_messenger_MieruClient_nativeFreeClient(JNIEnv *env, jobject thiz, jlong handle) {
    if (handle == 0) {
        return;
    }
    mieruclient_free((int64_t) handle);
    LOGI("nativeFreeClient");
}

JNIEXPORT jstring JNICALL
Java_org_telegram_messenger_MieruClient_nativeParseURL(JNIEnv *env, jclass clazz, jstring url) {
    if (url == NULL) {
        return NULL;
    }
    const char *urlStr = (*env)->GetStringUTFChars(env, url, NULL);
    if (urlStr == NULL) {
        return NULL;
    }
    const char *result = mieruclient_parse_url(urlStr);
    (*env)->ReleaseStringUTFChars(env, url, urlStr);
    if (result == NULL) {
        return NULL;
    }
    jstring value = (*env)->NewStringUTF(env, result);
    mieruclient_free_string((char *) result);
    return value;
}

JNIEXPORT jstring JNICALL
Java_org_telegram_messenger_MieruClient_nativeCreateURL(
    JNIEnv *env, jclass clazz, jstring serverAddress, jstring portSpec,
    jstring username, jstring password, jint mtu, jstring protocol) {
    const char *addrStr = serverAddress ? (*env)->GetStringUTFChars(env, serverAddress, NULL) : NULL;
    const char *portStr = portSpec ? (*env)->GetStringUTFChars(env, portSpec, NULL) : NULL;
    const char *userStr = username ? (*env)->GetStringUTFChars(env, username, NULL) : NULL;
    const char *passStr = password ? (*env)->GetStringUTFChars(env, password, NULL) : NULL;
    const char *protoStr = protocol ? (*env)->GetStringUTFChars(env, protocol, NULL) : NULL;
    const char *result = mieruclient_create_url(addrStr, portStr, userStr, passStr, (int32_t) mtu, protoStr);
    if (addrStr) (*env)->ReleaseStringUTFChars(env, serverAddress, addrStr);
    if (portStr) (*env)->ReleaseStringUTFChars(env, portSpec, portStr);
    if (userStr) (*env)->ReleaseStringUTFChars(env, username, userStr);
    if (passStr) (*env)->ReleaseStringUTFChars(env, password, passStr);
    if (protoStr) (*env)->ReleaseStringUTFChars(env, protocol, protoStr);
    if (result == NULL) {
        return NULL;
    }
    jstring value = (*env)->NewStringUTF(env, result);
    mieruclient_free_string((char *) result);
    return value;
}
