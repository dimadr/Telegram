#include <stdint.h>
#include <stddef.h>

typedef void (*mieru_connected_fn)(int32_t instance_num, uint64_t token, uint64_t dial_id, int32_t fd);
typedef void (*mieru_failed_fn)(int32_t instance_num, uint64_t token, uint64_t dial_id, int32_t error);

static mieru_connected_fn connected_callback = NULL;
static mieru_failed_fn failed_callback = NULL;

void mierujni_register_callbacks(uintptr_t connected, uintptr_t failed) {
    connected_callback = (mieru_connected_fn) connected;
    failed_callback = (mieru_failed_fn) failed;
}

void mierujni_call_connected(int32_t instance_num, uint64_t token, uint64_t dial_id, int32_t fd) {
    if (connected_callback != NULL) {
        connected_callback(instance_num, token, dial_id, fd);
    }
}

void mierujni_call_failed(int32_t instance_num, uint64_t token, uint64_t dial_id, int32_t error) {
    if (failed_callback != NULL) {
        failed_callback(instance_num, token, dial_id, error);
    }
}
