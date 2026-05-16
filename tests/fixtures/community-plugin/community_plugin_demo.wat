;; community_plugin_demo.wat — minimal WASM stub for CI (G-130).
;; Compiled form would be produced by: wat2wasm community_plugin_demo.wat
;; This file documents the expected module interface; the .wasm binary is
;; pre-built and committed as community_plugin_demo.wasm for offline CI.
(module
  ;; Import: forge-provided read_file host function (fs:read capability).
  (import "forge" "read_file"
    (func $read_file (param i32 i32) (result i32)))

  ;; scan(root_ptr i32, root_len i32, out_ptr i32) -> i32 (findings count)
  ;; Minimal stub: returns 0 findings without reading any files.
  (func (export "scan")
    (param $root_ptr i32)
    (param $root_len i32)
    (param $out_ptr  i32)
    (result i32)
    i32.const 0
  )

  ;; manifest() -> ptr to null-terminated JSON string in linear memory.
  (memory (export "memory") 1)
  (data (i32.const 0)
    "{\"name\":\"community-plugin-demo\",\"version\":\"0.1.0\",\"kind\":\"scanner\"}\00"
  )
  (func (export "manifest")
    (result i32)
    i32.const 0
  )
)
