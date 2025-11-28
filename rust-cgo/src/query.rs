// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

use std::ffi::{CStr, CString};
use std::os::raw::{c_char, c_float, c_int};

use arrow::ffi::FFI_ArrowArray;
use arrow::ffi::FFI_ArrowSchema;
use arrow_array::RecordBatch;
use futures::stream::BoxStream;
use futures::StreamExt;

use crate::arrow_ffi::export_record_batch_to_c;
use crate::error::Result;
use crate::RT;
use lancedb::query::{ExecutableQuery, Query as LanceQuery, QueryBase, VectorQuery};
use lancedb::DistanceType;

/// Opaque handle to a LanceDB query
/// Can be either a regular Query or a VectorQuery
pub enum QueryHandle {
    Plain(LanceQuery),
    Vector(VectorQuery),
}

impl QueryHandle {
    pub fn new(query: LanceQuery) -> Self {
        Self::Plain(query)
    }

    pub fn nearest_to(&mut self, vector: Vec<f32>) -> Result<()> {
        match self {
            QueryHandle::Plain(q) => {
                let vector_query = q.clone().nearest_to(vector)?;
                *self = QueryHandle::Vector(vector_query);
                Ok(())
            }
            QueryHandle::Vector(_) => Err(crate::error::Error::InvalidArgument {
                message: "nearest_to can only be called once on a query".to_string(),
                location: snafu::Location::new(file!(), line!(), column!()),
            }),
        }
    }

    pub fn distance_type(&mut self, distance_type: DistanceType) -> Result<()> {
        match self {
            QueryHandle::Vector(q) => {
                *self = QueryHandle::Vector(q.clone().distance_type(distance_type));
                Ok(())
            }
            QueryHandle::Plain(_) => Err(crate::error::Error::InvalidArgument {
                message: "distance_type can only be set on vector queries".to_string(),
                location: snafu::Location::new(file!(), line!(), column!()),
            }),
        }
    }

    pub fn limit(&mut self, limit: usize) -> Result<()> {
        match self {
            QueryHandle::Plain(q) => {
                *self = QueryHandle::Plain(q.clone().limit(limit));
                Ok(())
            }
            QueryHandle::Vector(q) => {
                *self = QueryHandle::Vector(q.clone().limit(limit));
                Ok(())
            }
        }
    }

    pub fn offset(&mut self, offset: usize) -> Result<()> {
        match self {
            QueryHandle::Plain(q) => {
                *self = QueryHandle::Plain(q.clone().offset(offset));
                Ok(())
            }
            QueryHandle::Vector(q) => {
                *self = QueryHandle::Vector(q.clone().offset(offset));
                Ok(())
            }
        }
    }

    pub fn filter(&mut self, filter: &str) -> Result<()> {
        match self {
            QueryHandle::Plain(q) => {
                *self = QueryHandle::Plain(q.clone().only_if(filter));
                Ok(())
            }
            QueryHandle::Vector(q) => {
                *self = QueryHandle::Vector(q.clone().only_if(filter));
                Ok(())
            }
        }
    }

    pub fn select(&mut self, columns: Vec<String>) -> Result<()> {
        match self {
            QueryHandle::Plain(q) => {
                *self =
                    QueryHandle::Plain(q.clone().select(lancedb::query::Select::columns(&columns)));
                Ok(())
            }
            QueryHandle::Vector(q) => {
                *self = QueryHandle::Vector(
                    q.clone().select(lancedb::query::Select::columns(&columns)),
                );
                Ok(())
            }
        }
    }

    pub fn execute(&self) -> Result<Vec<RecordBatch>> {
        let stream = match self {
            QueryHandle::Plain(q) => RT.block_on(q.execute())?,
            QueryHandle::Vector(q) => RT.block_on(q.execute())?,
        };

        let batches: Vec<RecordBatch> = RT.block_on(async {
            use futures::TryStreamExt;
            stream.try_collect::<Vec<_>>().await
        })?;
        Ok(batches)
    }

    pub fn execute_stream(&self) -> Result<BoxStream<'static, lancedb::Result<RecordBatch>>> {
        let stream = match self {
            QueryHandle::Plain(q) => RT.block_on(q.execute())?,
            QueryHandle::Vector(q) => RT.block_on(q.execute())?,
        };
        Ok(stream)
    }
}

pub struct QueryStreamHandle {
    stream: BoxStream<'static, lancedb::Result<RecordBatch>>,
}

// C API for queries

/// Create a new query for a table.
/// Returns a pointer to QueryHandle on success, null on failure.
#[no_mangle]
pub extern "C" fn lancedb_query_new(table: *const super::table::TableHandle) -> *mut QueryHandle {
    if table.is_null() {
        let error_msg = "table handle cannot be null";
        let c_error = CString::new(error_msg).unwrap();
        crate::lancedb_set_last_error(c_error.as_ptr());
        return std::ptr::null_mut();
    }

    let table = unsafe { &*table };
    let query = table.inner.query();
    let handle = QueryHandle::new(query);
    Box::into_raw(Box::new(handle))
}

/// Close a query and free resources.
#[no_mangle]
pub extern "C" fn lancedb_query_close(handle: *mut QueryHandle) {
    if !handle.is_null() {
        unsafe {
            let _ = Box::from_raw(handle);
        }
    }
}

/// Set the vector to search for nearest neighbors.
/// Returns 0 on success, -1 on failure.
#[no_mangle]
pub extern "C" fn lancedb_query_nearest_to(
    handle: *mut QueryHandle,
    vector: *const c_float,
    vector_len: c_int,
) -> c_int {
    if handle.is_null() || vector.is_null() || vector_len <= 0 {
        let error_msg = "handle, vector cannot be null and vector_len must be positive";
        let c_error = CString::new(error_msg).unwrap();
        crate::lancedb_set_last_error(c_error.as_ptr());
        return -1;
    }

    let query = unsafe { &mut *handle };

    // Convert C array to Rust Vec
    let vector_slice = unsafe { std::slice::from_raw_parts(vector, vector_len as usize) };
    let vector_vec = vector_slice.to_vec();

    match query.nearest_to(vector_vec) {
        Ok(_) => 0,
        Err(err) => {
            let error_msg = format!("{}", err);
            let c_error = CString::new(error_msg).unwrap();
            crate::lancedb_set_last_error(c_error.as_ptr());
            -1
        }
    }
}

/// Set the distance metric for the query.
/// distance_type: 0 = L2, 1 = Cosine, 2 = Dot
/// Returns 0 on success, -1 on failure.
#[no_mangle]
pub extern "C" fn lancedb_query_distance_type(
    handle: *mut QueryHandle,
    distance_type: c_int,
) -> c_int {
    if handle.is_null() {
        let error_msg = "handle cannot be null";
        let c_error = CString::new(error_msg).unwrap();
        crate::lancedb_set_last_error(c_error.as_ptr());
        return -1;
    }

    let query = unsafe { &mut *handle };

    let dist_type = match distance_type {
        0 => DistanceType::L2,
        1 => DistanceType::Cosine,
        2 => DistanceType::Dot,
        _ => {
            let error_msg = "invalid distance type: must be 0 (L2), 1 (Cosine), or 2 (Dot)";
            let c_error = CString::new(error_msg).unwrap();
            crate::lancedb_set_last_error(c_error.as_ptr());
            return -1;
        }
    };

    match query.distance_type(dist_type) {
        Ok(_) => 0,
        Err(err) => {
            let error_msg = format!("{}", err);
            let c_error = CString::new(error_msg).unwrap();
            crate::lancedb_set_last_error(c_error.as_ptr());
            -1
        }
    }
}

/// Set the maximum number of results to return.
/// Returns 0 on success, -1 on failure.
#[no_mangle]
pub extern "C" fn lancedb_query_limit(handle: *mut QueryHandle, limit: c_int) -> c_int {
    if handle.is_null() || limit < 0 {
        let error_msg = "handle cannot be null and limit must be non-negative";
        let c_error = CString::new(error_msg).unwrap();
        crate::lancedb_set_last_error(c_error.as_ptr());
        return -1;
    }

    let query = unsafe { &mut *handle };

    match query.limit(limit as usize) {
        Ok(_) => 0,
        Err(err) => {
            let error_msg = format!("{}", err);
            let c_error = CString::new(error_msg).unwrap();
            crate::lancedb_set_last_error(c_error.as_ptr());
            -1
        }
    }
}

/// Set the offset for query results.
/// Returns 0 on success, -1 on failure.
#[no_mangle]
pub extern "C" fn lancedb_query_offset(handle: *mut QueryHandle, offset: c_int) -> c_int {
    if handle.is_null() || offset < 0 {
        let error_msg = "handle cannot be null and offset must be non-negative";
        let c_error = CString::new(error_msg).unwrap();
        crate::lancedb_set_last_error(c_error.as_ptr());
        return -1;
    }

    let query = unsafe { &mut *handle };

    match query.offset(offset as usize) {
        Ok(_) => 0,
        Err(err) => {
            let error_msg = format!("{}", err);
            let c_error = CString::new(error_msg).unwrap();
            crate::lancedb_set_last_error(c_error.as_ptr());
            -1
        }
    }
}

/// Set a filter predicate for the query.
/// Returns 0 on success, -1 on failure.
#[no_mangle]
pub extern "C" fn lancedb_query_filter(handle: *mut QueryHandle, filter: *const c_char) -> c_int {
    if handle.is_null() || filter.is_null() {
        let error_msg = "handle and filter cannot be null";
        let c_error = CString::new(error_msg).unwrap();
        crate::lancedb_set_last_error(c_error.as_ptr());
        return -1;
    }

    let query = unsafe { &mut *handle };
    let c_str = unsafe { CStr::from_ptr(filter) };
    let filter_str = match c_str.to_str() {
        Ok(s) => s,
        Err(err) => {
            let error_msg = format!("invalid UTF-8 in filter: {}", err);
            let c_error = CString::new(error_msg).unwrap();
            crate::lancedb_set_last_error(c_error.as_ptr());
            return -1;
        }
    };

    match query.filter(filter_str) {
        Ok(_) => 0,
        Err(err) => {
            let error_msg = format!("{}", err);
            let c_error = CString::new(error_msg).unwrap();
            crate::lancedb_set_last_error(c_error.as_ptr());
            -1
        }
    }
}

/// Set the columns to select in the query results.
/// columns is a pointer to an array of C strings.
/// Returns 0 on success, -1 on failure.
#[no_mangle]
pub extern "C" fn lancedb_query_select(
    handle: *mut QueryHandle,
    columns: *const *const c_char,
    columns_len: c_int,
) -> c_int {
    if handle.is_null() || columns.is_null() || columns_len <= 0 {
        let error_msg = "handle, columns cannot be null and columns_len must be positive";
        let c_error = CString::new(error_msg).unwrap();
        crate::lancedb_set_last_error(c_error.as_ptr());
        return -1;
    }

    let query = unsafe { &mut *handle };

    // Convert C array of strings to Rust Vec<String>
    let columns_slice = unsafe { std::slice::from_raw_parts(columns, columns_len as usize) };
    let mut column_names = Vec::new();
    for &col_ptr in columns_slice {
        if col_ptr.is_null() {
            let error_msg = "column name cannot be null";
            let c_error = CString::new(error_msg).unwrap();
            crate::lancedb_set_last_error(c_error.as_ptr());
            return -1;
        }
        let c_str = unsafe { CStr::from_ptr(col_ptr) };
        let col_name = match c_str.to_str() {
            Ok(s) => s.to_string(),
            Err(err) => {
                let error_msg = format!("invalid UTF-8 in column name: {}", err);
                let c_error = CString::new(error_msg).unwrap();
                crate::lancedb_set_last_error(c_error.as_ptr());
                return -1;
            }
        };
        column_names.push(col_name);
    }

    match query.select(column_names) {
        Ok(_) => 0,
        Err(err) => {
            let error_msg = format!("{}", err);
            let c_error = CString::new(error_msg).unwrap();
            crate::lancedb_set_last_error(c_error.as_ptr());
            -1
        }
    }
}

/// Execute the query and return results as Arrow C Data Interface structures.
/// Returns 0 on success, -1 on failure.
#[no_mangle]
pub extern "C" fn lancedb_query_execute(
    handle: *const QueryHandle,
    arrays_out: *mut *mut FFI_ArrowArray,
    schemas_out: *mut *mut FFI_ArrowSchema,
    count_out: *mut c_int,
) -> c_int {
    if handle.is_null() || arrays_out.is_null() || schemas_out.is_null() || count_out.is_null() {
        let error_msg = "handle, arrays_out, schemas_out, and count_out cannot be null";
        let c_error = CString::new(error_msg).unwrap();
        crate::lancedb_set_last_error(c_error.as_ptr());
        return -1;
    }

    let query = unsafe { &*handle };

    // Execute the query
    let batches = match query.execute() {
        Ok(b) => b,
        Err(err) => {
            let error_msg = format!("{}", err);
            let c_error = CString::new(error_msg).unwrap();
            crate::lancedb_set_last_error(c_error.as_ptr());
            return -1;
        }
    };

    let num_batches = batches.len();

    if num_batches == 0 {
        // No results
        unsafe {
            *arrays_out = std::ptr::null_mut();
            *schemas_out = std::ptr::null_mut();
            *count_out = 0;
        }
        return 0;
    }

    // Allocate arrays for Arrow C structures
    let arrays_size = num_batches * std::mem::size_of::<FFI_ArrowArray>();
    let schemas_size = num_batches * std::mem::size_of::<FFI_ArrowSchema>();

    let arrays_ptr = unsafe { libc::malloc(arrays_size) as *mut FFI_ArrowArray };
    let schemas_ptr = unsafe { libc::malloc(schemas_size) as *mut FFI_ArrowSchema };

    if arrays_ptr.is_null() || schemas_ptr.is_null() {
        if !arrays_ptr.is_null() {
            unsafe { libc::free(arrays_ptr as *mut libc::c_void) };
        }
        if !schemas_ptr.is_null() {
            unsafe { libc::free(schemas_ptr as *mut libc::c_void) };
        }
        let error_msg = "failed to allocate memory for output arrays";
        let c_error = CString::new(error_msg).unwrap();
        crate::lancedb_set_last_error(c_error.as_ptr());
        return -1;
    }

    // Export each batch
    for (i, batch) in batches.iter().enumerate() {
        let array_ptr = unsafe { arrays_ptr.add(i) };
        let schema_ptr = unsafe { schemas_ptr.add(i) };

        if let Err(err) = unsafe { export_record_batch_to_c(batch, array_ptr, schema_ptr) } {
            // Clean up on error
            for j in 0..i {
                unsafe {
                    crate::arrow_ffi::lancedb_arrow_array_release(arrays_ptr.add(j));
                    crate::arrow_ffi::lancedb_arrow_schema_release(schemas_ptr.add(j));
                }
            }
            unsafe {
                libc::free(arrays_ptr as *mut libc::c_void);
                libc::free(schemas_ptr as *mut libc::c_void);
            }
            let error_msg = format!("{}", err);
            let c_error = CString::new(error_msg).unwrap();
            crate::lancedb_set_last_error(c_error.as_ptr());
            return -1;
        }
    }

    unsafe {
        *arrays_out = arrays_ptr;
        *schemas_out = schemas_ptr;
        *count_out = num_batches as c_int;
    }

    0
}

/// Execute the query and return a stream handle.
/// Returns a pointer to QueryStreamHandle on success, null on failure.
#[no_mangle]
pub extern "C" fn lancedb_query_execute_stream(handle: *const QueryHandle) -> *mut QueryStreamHandle {
    if handle.is_null() {
        let error_msg = "handle cannot be null";
        let c_error = CString::new(error_msg).unwrap();
        crate::lancedb_set_last_error(c_error.as_ptr());
        return std::ptr::null_mut();
    }

    let query = unsafe { &*handle };

    let stream = match query.execute_stream() {
        Ok(s) => s,
        Err(err) => {
            let error_msg = format!("{}", err);
            let c_error = CString::new(error_msg).unwrap();
            crate::lancedb_set_last_error(c_error.as_ptr());
            return std::ptr::null_mut();
        }
    };

    let handle = QueryStreamHandle { stream };
    Box::into_raw(Box::new(handle))
}

/// Get the next batch from the stream.
/// Returns 1 if a batch was returned, 0 if stream ended, -1 on error.
#[no_mangle]
pub extern "C" fn lancedb_stream_next(
    handle: *mut QueryStreamHandle,
    array_out: *mut FFI_ArrowArray,
    schema_out: *mut FFI_ArrowSchema,
) -> c_int {
    if handle.is_null() || array_out.is_null() || schema_out.is_null() {
        let error_msg = "handle, array_out, and schema_out cannot be null";
        let c_error = CString::new(error_msg).unwrap();
        crate::lancedb_set_last_error(c_error.as_ptr());
        return -1;
    }

    let stream_handle = unsafe { &mut *handle };

    let next_item = RT.block_on(async { stream_handle.stream.next().await });

    match next_item {
        Some(Ok(batch)) => {
            if let Err(err) = unsafe { export_record_batch_to_c(&batch, array_out, schema_out) } {
                let error_msg = format!("{}", err);
                let c_error = CString::new(error_msg).unwrap();
                crate::lancedb_set_last_error(c_error.as_ptr());
                return -1;
            }
            1
        }
        Some(Err(err)) => {
            let error_msg = format!("{}", err);
            let c_error = CString::new(error_msg).unwrap();
            crate::lancedb_set_last_error(c_error.as_ptr());
            -1
        }
        None => 0,
    }
}

/// Close the stream and free resources.
#[no_mangle]
pub extern "C" fn lancedb_stream_close(handle: *mut QueryStreamHandle) {
    if !handle.is_null() {
        unsafe {
            let _ = Box::from_raw(handle);
        }
    }
}
