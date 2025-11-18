// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

use std::ffi::{CStr, CString};
use std::os::raw::{c_char, c_int};
use std::sync::Arc;

use arrow::ffi::FFI_ArrowArray;
use arrow::ffi::FFI_ArrowSchema;
use arrow_array::{RecordBatch, RecordBatchIterator};
use arrow_schema::{DataType, Field, Schema};

use crate::arrow_ffi::import_record_batch_from_c;
use crate::error::Result;
use crate::{c_result, RT};
use lancedb::index::vector::IvfPqIndexBuilder;
use lancedb::index::{Index, IndexConfig};
use lancedb::query::{ExecutableQuery, QueryBase};
use lancedb::table::{AddDataMode, Table};
use lancedb::DistanceType;

/// Opaque handle to a LanceDB table
pub struct TableHandle {
    pub inner: Table,
}

impl TableHandle {
    pub fn open(connection: &super::connection::ConnectionHandle, name: &str) -> Result<Self> {
        let table = RT.block_on(connection.inner.open_table(name).execute())?;
        Ok(Self { inner: table })
    }

    #[allow(dead_code)] // Used by C API in future phases
    pub fn create(
        connection: &super::connection::ConnectionHandle,
        name: &str,
        schema: Arc<Schema>,
    ) -> Result<Self> {
        let table = RT.block_on(connection.inner.create_empty_table(name, schema).execute())?;
        Ok(Self { inner: table })
    }

    pub fn count_rows(&self) -> Result<i64> {
        let count = RT.block_on(self.inner.count_rows(None))?;
        Ok(count as i64)
    }

    pub fn add_data(&self, batch: RecordBatch, mode: AddDataMode) -> Result<()> {
        let schema = batch.schema();
        let reader = RecordBatchIterator::new(vec![Ok(batch)], schema);
        RT.block_on(self.inner.add(Box::new(reader)).mode(mode).execute())?;
        // The table reference remains valid - LanceDB uses internal versioning
        Ok(())
    }

    pub fn schema(&self) -> Result<Arc<Schema>> {
        let schema = RT.block_on(self.inner.schema())?;
        Ok(schema)
    }

    pub fn to_arrow(&self, limit: Option<i64>) -> Result<Vec<RecordBatch>> {
        // Create a query to read all data
        let query = self.inner.query();

        // Apply limit if specified
        let query = if let Some(lim) = limit {
            query.limit(lim as usize)
        } else {
            query
        };

        // Execute the query and collect results
        let stream = RT.block_on(query.execute())?;
        let batches: Vec<RecordBatch> = RT.block_on(async {
            use futures::TryStreamExt;
            stream.try_collect::<Vec<_>>().await
        })?;

        Ok(batches)
    }

    /// Create a vector index on a column
    pub fn create_index(
        &self,
        column: &str,
        index_type: &str,
        metric: DistanceType,
        num_partitions: Option<u32>,
        num_sub_vectors: Option<u32>,
        replace: bool,
    ) -> Result<()> {
        // Build the index based on type
        let index = match index_type.to_uppercase().as_str() {
            "IVF_PQ" => {
                let mut builder = IvfPqIndexBuilder::default().distance_type(metric);
                if let Some(partitions) = num_partitions {
                    builder = builder.num_partitions(partitions);
                }
                if let Some(sub_vectors) = num_sub_vectors {
                    builder = builder.num_sub_vectors(sub_vectors);
                }
                Index::IvfPq(builder)
            }
            "AUTO" => Index::Auto,
            _ => {
                return Err(crate::error::Error::InvalidArgument {
                    message: format!("Unsupported index type: {}", index_type),
                    location: snafu::Location::new(file!(), line!(), column!()),
                });
            }
        };

        // Create the index
        RT.block_on(
            self.inner
                .create_index(&[column], index)
                .replace(replace)
                .execute(),
        )?;
        Ok(())
    }

    /// List all indices on the table
    pub fn list_indices(&self) -> Result<Vec<IndexConfig>> {
        let indices = RT.block_on(self.inner.list_indices())?;
        Ok(indices)
    }
}

// C API for tables

/// Open an existing table.
/// Returns a pointer to TableHandle on success, null on failure.
#[no_mangle]
pub extern "C" fn lancedb_table_open(
    connection: *const super::connection::ConnectionHandle,
    name: *const c_char,
) -> *mut TableHandle {
    if connection.is_null() || name.is_null() {
        let error_msg = "connection and name cannot be null";
        let c_error = CString::new(error_msg).unwrap();
        crate::lancedb_set_last_error(c_error.as_ptr());
        return std::ptr::null_mut();
    }

    let connection = unsafe { &*connection };
    let c_str = unsafe { CStr::from_ptr(name) };
    let table_name = c_result!(c_str.to_str());

    let handle = c_result!(TableHandle::open(connection, table_name));
    Box::into_raw(Box::new(handle))
}

/// Create a new table. For now, creates an empty table.
/// Returns a pointer to TableHandle on success, null on failure.
#[no_mangle]
pub extern "C" fn lancedb_table_create(
    connection: *const super::connection::ConnectionHandle,
    name: *const c_char,
) -> *mut TableHandle {
    if connection.is_null() || name.is_null() {
        let error_msg = "connection and name cannot be null";
        let c_error = CString::new(error_msg).unwrap();
        crate::lancedb_set_last_error(c_error.as_ptr());
        return std::ptr::null_mut();
    }

    let connection = unsafe { &*connection };
    let c_str = unsafe { CStr::from_ptr(name) };
    let table_name = c_result!(c_str.to_str());

    // Create a simple schema for demonstration
    let schema = Arc::new(Schema::new(vec![
        Field::new("id", DataType::Int32, false),
        Field::new("text", DataType::Utf8, true),
    ]));

    let table = c_result!(RT.block_on(
        connection
            .inner
            .create_empty_table(table_name, schema)
            .execute()
    ));
    let handle = TableHandle { inner: table };
    Box::into_raw(Box::new(handle))
}

/// Close a table and free resources.
#[no_mangle]
pub extern "C" fn lancedb_table_close(handle: *mut TableHandle) {
    if !handle.is_null() {
        let _ = unsafe { Box::from_raw(handle) };
    }
}

/// Get the number of rows in a table.
/// Returns the count on success, -1 on failure.
#[no_mangle]
pub extern "C" fn lancedb_table_count_rows(handle: *const TableHandle) -> i64 {
    if handle.is_null() {
        let error_msg = "table handle cannot be null";
        let c_error = CString::new(error_msg).unwrap();
        crate::lancedb_set_last_error(c_error.as_ptr());
        return -1;
    }

    let table = unsafe { &*handle };
    match table.count_rows() {
        Ok(count) => count,
        Err(err) => {
            let error_msg = format!("{}", err);
            let c_error = CString::new(error_msg).unwrap();
            crate::lancedb_set_last_error(c_error.as_ptr());
            -1
        }
    }
}

/// Add data to a table from Arrow C Data Interface structures.
/// Returns 0 on success, -1 on failure.
/// mode: 0 = Append, 1 = Overwrite
#[no_mangle]
pub extern "C" fn lancedb_table_add(
    handle: *const TableHandle,
    array: *mut FFI_ArrowArray,
    schema: *mut FFI_ArrowSchema,
    mode: c_int,
) -> c_int {
    if handle.is_null() || array.is_null() || schema.is_null() {
        let error_msg = "table handle, array, and schema cannot be null";
        let c_error = CString::new(error_msg).unwrap();
        crate::lancedb_set_last_error(c_error.as_ptr());
        return -1;
    }

    let table = unsafe { &*handle };

    // Import the record batch from C
    let batch = match unsafe { import_record_batch_from_c(array, schema) } {
        Ok(b) => b,
        Err(err) => {
            let error_msg = format!("{}", err);
            let c_error = CString::new(error_msg).unwrap();
            crate::lancedb_set_last_error(c_error.as_ptr());
            return -1;
        }
    };

    // Determine the mode
    let add_mode = match mode {
        0 => AddDataMode::Append,
        1 => AddDataMode::Overwrite,
        _ => {
            let error_msg = "invalid mode: must be 0 (Append) or 1 (Overwrite)";
            let c_error = CString::new(error_msg).unwrap();
            crate::lancedb_set_last_error(c_error.as_ptr());
            return -1;
        }
    };

    // Add the data
    match table.add_data(batch, add_mode) {
        Ok(_) => 0,
        Err(err) => {
            let error_msg = format!("{}", err);
            let c_error = CString::new(error_msg).unwrap();
            crate::lancedb_set_last_error(c_error.as_ptr());
            -1
        }
    }
}

/// Get the schema of a table as Arrow C Data Interface structure.
/// Returns 0 on success, -1 on failure.
#[no_mangle]
pub extern "C" fn lancedb_table_schema(
    handle: *const TableHandle,
    schema_out: *mut FFI_ArrowSchema,
) -> c_int {
    if handle.is_null() || schema_out.is_null() {
        let error_msg = "table handle and schema_out cannot be null";
        let c_error = CString::new(error_msg).unwrap();
        crate::lancedb_set_last_error(c_error.as_ptr());
        return -1;
    }

    let table = unsafe { &*handle };

    // Get the schema
    let schema = match table.schema() {
        Ok(s) => s,
        Err(err) => {
            let error_msg = format!("{}", err);
            let c_error = CString::new(error_msg).unwrap();
            crate::lancedb_set_last_error(c_error.as_ptr());
            return -1;
        }
    };

    // Export to C
    unsafe {
        match crate::arrow_ffi::export_schema_to_c(&schema, schema_out) {
            Ok(_) => 0,
            Err(err) => {
                let error_msg = format!("{}", err);
                let c_error = CString::new(error_msg).unwrap();
                crate::lancedb_set_last_error(c_error.as_ptr());
                -1
            }
        }
    }
}

/// Create a table with a custom schema from Arrow C Data Interface structure.
/// Returns a pointer to TableHandle on success, null on failure.
#[no_mangle]
pub extern "C" fn lancedb_table_create_with_schema(
    connection: *const super::connection::ConnectionHandle,
    name: *const c_char,
    schema: *mut FFI_ArrowSchema,
) -> *mut TableHandle {
    if connection.is_null() || name.is_null() || schema.is_null() {
        let error_msg = "connection, name, and schema cannot be null";
        let c_error = CString::new(error_msg).unwrap();
        crate::lancedb_set_last_error(c_error.as_ptr());
        return std::ptr::null_mut();
    }

    let connection = unsafe { &*connection };
    let c_str = unsafe { CStr::from_ptr(name) };
    let table_name = c_result!(c_str.to_str());

    // Import the schema
    let imported_schema = c_result!(unsafe { crate::arrow_ffi::import_schema_from_c(schema) });

    let handle = c_result!(TableHandle::create(
        connection,
        table_name,
        Arc::new(imported_schema)
    ));
    Box::into_raw(Box::new(handle))
}

/// Read data from a table as Arrow C Data Interface structures.
/// Returns the number of batches on success, -1 on failure.
/// limit: maximum number of rows to read (-1 for no limit)
/// arrays_out and schemas_out will be populated with arrays of Arrow C structures.
/// Caller is responsible for freeing the arrays and schemas.
#[no_mangle]
pub extern "C" fn lancedb_table_to_arrow(
    handle: *const TableHandle,
    limit: i64,
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

    let table = unsafe { &*handle };

    // Read the data
    let limit_opt = if limit < 0 { None } else { Some(limit) };
    let batches = match table.to_arrow(limit_opt) {
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
        // No data - return empty arrays
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

        if let Err(err) =
            unsafe { crate::arrow_ffi::export_record_batch_to_c(batch, array_ptr, schema_ptr) }
        {
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

/// Create an index on a table column.
/// Returns 0 on success, -1 on failure.
///
/// # Parameters
/// * `handle` - The table handle
/// * `column` - The column name to index
/// * `index_type` - The type of index ("IVF_PQ", "AUTO")
/// * `metric` - Distance metric (0=L2, 1=Cosine, 2=Dot)
/// * `num_partitions` - Number of IVF partitions (0 for default)
/// * `num_sub_vectors` - Number of PQ sub-vectors (0 for default)
/// * `replace` - Whether to replace existing index
#[no_mangle]
pub extern "C" fn lancedb_table_create_index(
    handle: *const TableHandle,
    column: *const c_char,
    index_type: *const c_char,
    metric: c_int,
    num_partitions: c_int,
    num_sub_vectors: c_int,
    replace: bool,
) -> c_int {
    if handle.is_null() || column.is_null() || index_type.is_null() {
        let error_msg = "table handle, column, and index_type cannot be null";
        let c_error = CString::new(error_msg).unwrap();
        crate::lancedb_set_last_error(c_error.as_ptr());
        return -1;
    }

    let table = unsafe { &*handle };
    let column_str = match unsafe { CStr::from_ptr(column) }.to_str() {
        Ok(s) => s,
        Err(err) => {
            let error_msg = format!("{}", err);
            let c_error = CString::new(error_msg).unwrap();
            crate::lancedb_set_last_error(c_error.as_ptr());
            return -1;
        }
    };
    let index_type_str = match unsafe { CStr::from_ptr(index_type) }.to_str() {
        Ok(s) => s,
        Err(err) => {
            let error_msg = format!("{}", err);
            let c_error = CString::new(error_msg).unwrap();
            crate::lancedb_set_last_error(c_error.as_ptr());
            return -1;
        }
    };

    // Convert metric int to DistanceType
    let distance_type = match metric {
        0 => DistanceType::L2,
        1 => DistanceType::Cosine,
        2 => DistanceType::Dot,
        _ => {
            let error_msg = "Invalid distance metric. Use 0=L2, 1=Cosine, 2=Dot";
            let c_error = CString::new(error_msg).unwrap();
            crate::lancedb_set_last_error(c_error.as_ptr());
            return -1;
        }
    };

    let partitions = if num_partitions > 0 {
        Some(num_partitions as u32)
    } else {
        None
    };

    let sub_vectors = if num_sub_vectors > 0 {
        Some(num_sub_vectors as u32)
    } else {
        None
    };

    match table.create_index(
        column_str,
        index_type_str,
        distance_type,
        partitions,
        sub_vectors,
        replace,
    ) {
        Ok(_) => 0,
        Err(err) => {
            let error_msg = format!("{}", err);
            let c_error = CString::new(error_msg).unwrap();
            crate::lancedb_set_last_error(c_error.as_ptr());
            -1
        }
    }
}

/// List all indices on a table.
/// Returns the number of indices on success, -1 on failure.
/// indices_json_out will be populated with a JSON string containing the indices.
/// Caller is responsible for freeing the string with lancedb_free_string.
#[no_mangle]
pub extern "C" fn lancedb_table_list_indices(
    handle: *const TableHandle,
    indices_json_out: *mut *mut c_char,
) -> c_int {
    if handle.is_null() || indices_json_out.is_null() {
        let error_msg = "table handle and indices_json_out cannot be null";
        let c_error = CString::new(error_msg).unwrap();
        crate::lancedb_set_last_error(c_error.as_ptr());
        return -1;
    }

    let table = unsafe { &*handle };
    let indices = match table.list_indices() {
        Ok(idx) => idx,
        Err(err) => {
            let error_msg = format!("{}", err);
            let c_error = CString::new(error_msg).unwrap();
            crate::lancedb_set_last_error(c_error.as_ptr());
            return -1;
        }
    };

    // Manually build JSON array from IndexConfig structs
    let json_objects: Vec<String> = indices
        .iter()
        .map(|idx| {
            format!(
                r#"{{"name":"{}","type":"{:?}","columns":[{}]}}"#,
                idx.name,
                idx.index_type,
                idx.columns
                    .iter()
                    .map(|c| format!(r#""{}""#, c))
                    .collect::<Vec<_>>()
                    .join(",")
            )
        })
        .collect();

    let json = format!("[{}]", json_objects.join(","));

    let c_string = match CString::new(json) {
        Ok(s) => s,
        Err(err) => {
            let error_msg = format!("{}", err);
            let c_error = CString::new(error_msg).unwrap();
            crate::lancedb_set_last_error(c_error.as_ptr());
            return -1;
        }
    };

    unsafe {
        *indices_json_out = c_string.into_raw();
    }

    indices.len() as c_int
}
