// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

use lazy_static::lazy_static;
use std::ffi::{CStr, CString};
use std::os::raw::{c_char, c_int};

// Error handling macros similar to JNI
#[macro_export]
macro_rules! c_result {
    ($result:expr) => {
        match $result {
            Ok(value) => value,
            Err(err) => {
                let error_msg = format!("{}", err);
                let c_error = std::ffi::CString::new(error_msg).unwrap();
                $crate::lancedb_set_last_error(c_error.as_ptr());
                return std::ptr::null_mut();
            }
        }
    };
}

#[macro_export]
macro_rules! c_result_int {
    ($result:expr) => {
        match $result {
            Ok(value) => value as std::os::raw::c_int,
            Err(err) => {
                let error_msg = format!("{}", err);
                let c_error = std::ffi::CString::new(error_msg).unwrap();
                $crate::lancedb_set_last_error(c_error.as_ptr());
                return -1;
            }
        }
    };
}

pub mod arrow_ffi;
mod connection;
pub mod error;
mod query;
mod table;

pub use error::{Error, Result};

lazy_static! {
    static ref RT: tokio::runtime::Runtime = tokio::runtime::Builder::new_multi_thread()
        .enable_all()
        .build()
        .expect("Failed to create tokio runtime");
}

// C API functions

/// Initialize the LanceDB runtime. Must be called before any other functions.
#[no_mangle]
pub extern "C" fn lancedb_init() -> c_int {
    // Runtime is initialized lazily when first accessed
    0
}

/// Clean up resources. Call this when done with LanceDB.
#[no_mangle]
pub extern "C" fn lancedb_cleanup() {
    // The lazy static will be dropped when the program exits
}

/// Get the last error message. Returns a pointer to a C string that must be freed with lancedb_free_string.
#[no_mangle]
pub extern "C" fn lancedb_get_last_error() -> *const c_char {
    LAST_ERROR.with(|e| {
        e.borrow()
            .as_ref()
            .map(|s| s.as_ptr())
            .unwrap_or(std::ptr::null())
    })
}

/// Free a string returned by the C API.
#[no_mangle]
pub extern "C" fn lancedb_free_string(s: *mut c_char) {
    if !s.is_null() {
        unsafe {
            let _ = CString::from_raw(s);
        }
    }
}

// Thread-local storage for error messages
thread_local! {
    static LAST_ERROR: std::cell::RefCell<Option<CString>> = std::cell::RefCell::new(None);
}

#[no_mangle]
pub extern "C" fn lancedb_set_last_error(error: *const c_char) {
    if error.is_null() {
        LAST_ERROR.with(|e| *e.borrow_mut() = None);
        return;
    }

    let c_str = unsafe { CStr::from_ptr(error) };
    let error_string = c_str.to_string_lossy().into_owned();
    let c_error = CString::new(error_string).unwrap();

    LAST_ERROR.with(|e| *e.borrow_mut() = Some(c_error));
}
