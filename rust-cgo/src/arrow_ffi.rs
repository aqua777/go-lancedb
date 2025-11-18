// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

//! Arrow C Data Interface implementation for CGO bindings
//!
//! This module provides conversion functions between Arrow RecordBatch and
//! the C Data Interface structures (ArrowArray and ArrowSchema).

use std::sync::Arc;

use arrow::ffi::{from_ffi, FFI_ArrowArray, FFI_ArrowSchema};
use arrow_array::{Array, RecordBatch, StructArray};
use arrow_schema::Schema;

use crate::error::Result;

/// Import a RecordBatch from C Data Interface structures
///
/// # Safety
///
/// The caller must ensure that:
/// - `array` and `schema` pointers are valid
/// - The memory they point to follows the Arrow C Data Interface specification
/// - The data remains valid for the duration of this function call
pub unsafe fn import_record_batch_from_c(
    array: *mut FFI_ArrowArray,
    schema: *mut FFI_ArrowSchema,
) -> Result<RecordBatch> {
    if array.is_null() || schema.is_null() {
        return Err(crate::error::Error::InvalidArgument {
            message: "array and schema pointers cannot be null".to_string(),
            location: snafu::Location::new(file!(), line!(), column!()),
        });
    }

    // Import schema from the C structure
    let ffi_schema_ref = &*schema;
    let _imported_schema =
        Arc::new(
            Schema::try_from(ffi_schema_ref).map_err(|e| crate::error::Error::Arrow {
                message: format!("Failed to convert schema: {}", e),
                location: snafu::Location::new(file!(), line!(), column!()),
            })?,
        );

    // Import array data from the C structure
    // We need to move the FFI_ArrowArray, so we read it and replace with zeroed memory
    let ffi_array = std::ptr::read(array);
    std::ptr::write(array, std::mem::zeroed());

    let array_data =
        from_ffi(ffi_array, ffi_schema_ref).map_err(|e| crate::error::Error::Arrow {
            message: format!("Failed to convert array data: {}", e),
            location: snafu::Location::new(file!(), line!(), column!()),
        })?;

    // Convert ArrayData to StructArray, then to RecordBatch
    let struct_array = StructArray::from(array_data);
    let batch = RecordBatch::from(&struct_array);
    Ok(batch)
}

/// Export a RecordBatch to C Data Interface structures
///
/// # Safety
///
/// The caller must ensure that:
/// - `array_out` and `schema_out` point to valid, uninitialized memory
/// - The caller takes ownership of the exported structures and must call their release callbacks
pub unsafe fn export_record_batch_to_c(
    batch: &RecordBatch,
    array_out: *mut FFI_ArrowArray,
    schema_out: *mut FFI_ArrowSchema,
) -> Result<()> {
    if array_out.is_null() || schema_out.is_null() {
        return Err(crate::error::Error::InvalidArgument {
            message: "array_out and schema_out pointers cannot be null".to_string(),
            location: snafu::Location::new(file!(), line!(), column!()),
        });
    }

    // Convert RecordBatch to StructArray for FFI export
    // Clone the batch since we need ownership for the conversion
    let struct_array = StructArray::from(batch.clone());

    // Export schema
    let ffi_schema = FFI_ArrowSchema::try_from(struct_array.data_type()).map_err(|e| {
        crate::error::Error::Arrow {
            message: format!("Failed to export schema: {}", e),
            location: snafu::Location::new(file!(), line!(), column!()),
        }
    })?;

    std::ptr::write(schema_out, ffi_schema);

    // Export array data
    let ffi_array = FFI_ArrowArray::new(&struct_array.to_data());
    std::ptr::write(array_out, ffi_array);

    Ok(())
}

/// Export a Schema to C Data Interface structure
///
/// # Safety
///
/// The caller must ensure that:
/// - `schema_out` points to valid, uninitialized memory
/// - The caller takes ownership and must call the release callback
pub unsafe fn export_schema_to_c(schema: &Schema, schema_out: *mut FFI_ArrowSchema) -> Result<()> {
    if schema_out.is_null() {
        return Err(crate::error::Error::InvalidArgument {
            message: "schema_out pointer cannot be null".to_string(),
            location: snafu::Location::new(file!(), line!(), column!()),
        });
    }

    let ffi_schema = FFI_ArrowSchema::try_from(schema).map_err(|e| crate::error::Error::Arrow {
        message: format!("Failed to export schema: {}", e),
        location: snafu::Location::new(file!(), line!(), column!()),
    })?;

    std::ptr::write(schema_out, ffi_schema);
    Ok(())
}

/// Import a Schema from C Data Interface structure
///
/// # Safety
///
/// The caller must ensure that:
/// - `schema` pointer is valid
/// - The memory follows the Arrow C Data Interface specification
pub unsafe fn import_schema_from_c(schema: *mut FFI_ArrowSchema) -> Result<Schema> {
    if schema.is_null() {
        return Err(crate::error::Error::InvalidArgument {
            message: "schema pointer cannot be null".to_string(),
            location: snafu::Location::new(file!(), line!(), column!()),
        });
    }

    let ffi_schema = &*schema;

    Schema::try_from(ffi_schema).map_err(|e| crate::error::Error::Arrow {
        message: format!("Failed to convert schema: {}", e),
        location: snafu::Location::new(file!(), line!(), column!()),
    })
}

// C API functions

/// Free an Arrow C Data Interface ArrowArray structure
///
/// Note: The FFI structures handle their own cleanup through release callbacks.
/// This function is provided for completeness but typically you don't need to call it
/// as the structures will be released when dropped.
///
/// # Safety
///
/// The caller must ensure the array pointer is valid
#[no_mangle]
pub unsafe extern "C" fn lancedb_arrow_array_release(array: *mut FFI_ArrowArray) {
    if !array.is_null() {
        // Drop the array, which will call its release callback
        let _ = std::ptr::read(array);
    }
}

/// Free an Arrow C Data Interface ArrowSchema structure
///
/// Note: The FFI structures handle their own cleanup through release callbacks.
/// This function is provided for completeness but typically you don't need to call it
/// as the structures will be released when dropped.
///
/// # Safety
///
/// The caller must ensure the schema pointer is valid
#[no_mangle]
pub unsafe extern "C" fn lancedb_arrow_schema_release(schema: *mut FFI_ArrowSchema) {
    if !schema.is_null() {
        // Drop the schema, which will call its release callback
        let _ = std::ptr::read(schema);
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use arrow_array::{Int32Array, RecordBatch, StringArray};
    use arrow_schema::{DataType, Field, Schema};
    use std::sync::Arc;

    #[test]
    fn test_roundtrip_record_batch() {
        // Create a simple RecordBatch
        let schema = Arc::new(Schema::new(vec![
            Field::new("id", DataType::Int32, false),
            Field::new("name", DataType::Utf8, false),
        ]));

        let id_array = Int32Array::from(vec![1, 2, 3]);
        let name_array = StringArray::from(vec!["Alice", "Bob", "Charlie"]);

        let batch = RecordBatch::try_new(
            schema.clone(),
            vec![Arc::new(id_array), Arc::new(name_array)],
        )
        .unwrap();

        // Export to C
        let mut array_out = std::mem::MaybeUninit::<FFI_ArrowArray>::uninit();
        let mut schema_out = std::mem::MaybeUninit::<FFI_ArrowSchema>::uninit();

        unsafe {
            export_record_batch_to_c(&batch, array_out.as_mut_ptr(), schema_out.as_mut_ptr())
                .unwrap();

            // Import back
            let imported_batch =
                import_record_batch_from_c(array_out.as_mut_ptr(), schema_out.as_mut_ptr())
                    .unwrap();

            // Verify the data matches
            assert_eq!(imported_batch.num_rows(), 3);
            assert_eq!(imported_batch.num_columns(), 2);
            assert_eq!(imported_batch.schema(), schema);
        }
    }

    #[test]
    fn test_roundtrip_schema() {
        let schema = Schema::new(vec![
            Field::new("id", DataType::Int32, false),
            Field::new("value", DataType::Float64, true),
        ]);

        let mut schema_out = std::mem::MaybeUninit::<FFI_ArrowSchema>::uninit();

        unsafe {
            export_schema_to_c(&schema, schema_out.as_mut_ptr()).unwrap();
            let imported_schema = import_schema_from_c(schema_out.as_mut_ptr()).unwrap();

            assert_eq!(imported_schema, schema);
        }
    }
}
