// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

use std::ffi::{CStr, CString};
use std::os::raw::{c_char, c_int};

use crate::error::Result;
use crate::{c_result, RT};
use lancedb::connection::{connect, Connection};

/// Opaque handle to a LanceDB connection
#[derive(Clone)]
pub struct ConnectionHandle {
    pub inner: Connection,
}

impl ConnectionHandle {
    pub fn create(dataset_uri: &str) -> Result<Self> {
        let inner = RT.block_on(connect(dataset_uri).execute())?;
        Ok(Self { inner })
    }

    pub fn table_names(
        &self,
        start_after: Option<String>,
        limit: Option<i32>,
    ) -> Result<Vec<String>> {
        let mut op = self.inner.table_names();
        if let Some(start_after) = start_after {
            op = op.start_after(start_after);
        }
        if let Some(limit) = limit {
            op = op.limit(limit as u32);
        }
        Ok(RT.block_on(op.execute())?)
    }
}

// C API for connections

/// Create a new database connection.
/// Returns a pointer to ConnectionHandle on success, null on failure.
/// Use lancedb_get_last_error() to get error details.
#[no_mangle]
pub extern "C" fn lancedb_connect(dataset_uri: *const c_char) -> *mut ConnectionHandle {
    if dataset_uri.is_null() {
        let error_msg = "dataset_uri cannot be null";
        let c_error = CString::new(error_msg).unwrap();
        crate::lancedb_set_last_error(c_error.as_ptr());
        return std::ptr::null_mut();
    }

    let c_str = unsafe { CStr::from_ptr(dataset_uri) };
    let uri = c_result!(c_str.to_str());

    let handle = c_result!(ConnectionHandle::create(uri));
    Box::into_raw(Box::new(handle))
}

/// Close a database connection and free resources.
#[no_mangle]
pub extern "C" fn lancedb_connection_close(handle: *mut ConnectionHandle) {
    if !handle.is_null() {
        unsafe {
            let _ = Box::from_raw(handle);
        }
    }
}

/// Get table names from the database.
/// Returns the number of table names on success, -1 on failure.
/// table_names_out will be populated with a null-terminated array of C strings.
/// Caller is responsible for freeing the strings with lancedb_free_string_array.
#[no_mangle]
pub extern "C" fn lancedb_connection_table_names(
    handle: *const ConnectionHandle,
    start_after: *const c_char,
    limit: c_int,
    table_names_out: *mut *mut *mut c_char,
    count_out: *mut c_int,
) -> c_int {
    if handle.is_null() {
        let error_msg = "connection handle cannot be null";
        let c_error = CString::new(error_msg).unwrap();
        crate::lancedb_set_last_error(c_error.as_ptr());
        return -1;
    }

    let connection = unsafe { &*handle };

    let start_after_opt = if start_after.is_null() {
        None
    } else {
        let c_str = unsafe { CStr::from_ptr(start_after) };
        let s = match c_str.to_str() {
            Ok(s) => s,
            Err(err) => {
                let error_msg = format!("{}", err);
                let c_error = CString::new(error_msg).unwrap();
                crate::lancedb_set_last_error(c_error.as_ptr());
                return -1;
            }
        };
        Some(s.to_string())
    };

    let limit_opt = if limit <= 0 { None } else { Some(limit) };

    let table_names = match connection.table_names(start_after_opt, limit_opt) {
        Ok(names) => names,
        Err(err) => {
            let error_msg = format!("{}", err);
            let c_error = CString::new(error_msg).unwrap();
            crate::lancedb_set_last_error(c_error.as_ptr());
            return -1;
        }
    };

    // Convert Rust strings to C strings
    let mut c_strings: Vec<*mut c_char> = Vec::with_capacity(table_names.len());
    for name in table_names {
        let c_string = match CString::new(name) {
            Ok(s) => s,
            Err(err) => {
                let error_msg = format!("{}", err);
                let c_error = CString::new(error_msg).unwrap();
                crate::lancedb_set_last_error(c_error.as_ptr());
                return -1;
            }
        };
        c_strings.push(c_string.into_raw());
    }

    // Add null terminator
    c_strings.push(std::ptr::null_mut());

    // Allocate array and copy pointers
    let array_size = c_strings.len() * std::mem::size_of::<*mut c_char>();
    let array_ptr = unsafe { libc::malloc(array_size) as *mut *mut c_char };
    if array_ptr.is_null() {
        return -1;
    }

    unsafe {
        std::ptr::copy_nonoverlapping(c_strings.as_ptr(), array_ptr, c_strings.len());
        *table_names_out = array_ptr;
        *count_out = (c_strings.len() - 1) as c_int; // Don't count null terminator
    }

    0
}

/// Free an array of C strings allocated by lancedb_connection_table_names.
#[no_mangle]
pub extern "C" fn lancedb_free_string_array(array: *mut *mut c_char, count: c_int) {
    if array.is_null() {
        return;
    }

    // Free individual strings
    for i in 0..count {
        let string_ptr = unsafe { *array.offset(i as isize) };
        if !string_ptr.is_null() {
            unsafe {
                let _ = CString::from_raw(string_ptr);
            }
        }
    }

    // Free the array
    unsafe {
        libc::free(array as *mut libc::c_void);
    }
}
