// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

use std::str::Utf8Error;

use arrow_schema::ArrowError;
use serde_json::Error as JsonError;
use snafu::{Location, Snafu};

type BoxedError = Box<dyn std::error::Error + Send + Sync + 'static>;

#[derive(Debug, Snafu)]
#[snafu(visibility(pub))]
pub enum Error {
    #[snafu(display("Invalid argument: {message}, {location}"))]
    InvalidArgument { message: String, location: Location },
    #[snafu(display("IO error: {source}, {location}"))]
    IO {
        source: BoxedError,
        location: Location,
    },
    #[snafu(display("Arrow error: {message}, {location}"))]
    Arrow { message: String, location: Location },
    #[snafu(display("Index error: {message}, {location}"))]
    Index { message: String, location: Location },
    #[snafu(display("JSON error: {message}, {location}"))]
    JSON { message: String, location: Location },
    #[snafu(display("Dataset at path {path} was not found, {location}"))]
    DatasetNotFound { path: String, location: Location },
    #[snafu(display("Dataset already exists: {uri}, {location}"))]
    DatasetAlreadyExists { uri: String, location: Location },
    #[snafu(display("Table '{name}' already exists"))]
    TableAlreadyExists { name: String },
    #[snafu(display("Table '{name}' was not found: {source}"))]
    TableNotFound {
        name: String,
        source: Box<dyn std::error::Error + Send + Sync>,
    },
    #[snafu(display("Invalid table name '{name}': {reason}"))]
    InvalidTableName { name: String, reason: String },
    #[snafu(display("Embedding function '{name}' was not found: {reason}, {location}"))]
    EmbeddingFunctionNotFound {
        name: String,
        reason: String,
        location: Location,
    },
    #[snafu(display("Other Lance error: {message}, {location}"))]
    OtherLance { message: String, location: Location },
    #[snafu(display("Other LanceDB error: {message}, {location}"))]
    OtherLanceDB { message: String, location: Location },
    #[snafu(display("Null pointer error, {location}"))]
    NullPointer { location: Location },
    #[snafu(display("UTF-8 conversion error: {message}, {location}"))]
    Utf8Error { message: String, location: Location },
}

pub type Result<T> = std::result::Result<T, Error>;

impl From<Utf8Error> for Error {
    #[track_caller]
    fn from(source: Utf8Error) -> Self {
        Self::Utf8Error {
            message: source.to_string(),
            location: std::panic::Location::caller().to_snafu_location(),
        }
    }
}

impl From<ArrowError> for Error {
    #[track_caller]
    fn from(source: ArrowError) -> Self {
        Self::Arrow {
            message: source.to_string(),
            location: std::panic::Location::caller().to_snafu_location(),
        }
    }
}

impl From<JsonError> for Error {
    #[track_caller]
    fn from(source: JsonError) -> Self {
        Self::JSON {
            message: source.to_string(),
            location: std::panic::Location::caller().to_snafu_location(),
        }
    }
}

impl From<lance::Error> for Error {
    #[track_caller]
    fn from(source: lance::Error) -> Self {
        match source {
            lance::Error::DatasetNotFound {
                path,
                source: _,
                location,
            } => Self::DatasetNotFound { path, location },
            lance::Error::DatasetAlreadyExists { uri, location } => {
                Self::DatasetAlreadyExists { uri, location }
            }
            lance::Error::IO { source, location } => Self::IO { source, location },
            lance::Error::Arrow { message, location } => Self::Arrow { message, location },
            lance::Error::Index { message, location } => Self::Index { message, location },
            lance::Error::InvalidInput { source, location } => Self::InvalidArgument {
                message: source.to_string(),
                location,
            },
            _ => Self::OtherLance {
                message: source.to_string(),
                location: std::panic::Location::caller().to_snafu_location(),
            },
        }
    }
}

impl From<lancedb::Error> for Error {
    #[track_caller]
    fn from(source: lancedb::Error) -> Self {
        match source {
            lancedb::Error::InvalidTableName { name, reason } => {
                Self::InvalidTableName { name, reason }
            }
            lancedb::Error::InvalidInput { message } => Self::InvalidArgument {
                message,
                location: std::panic::Location::caller().to_snafu_location(),
            },
            lancedb::Error::TableNotFound { name } => Self::TableNotFound { 
                name: name.clone(),
                source: Box::new(std::io::Error::new(std::io::ErrorKind::NotFound, format!("Table '{}' not found", name))),
            },
            lancedb::Error::TableAlreadyExists { name } => Self::TableAlreadyExists { name },
            lancedb::Error::EmbeddingFunctionNotFound { name, reason } => {
                Self::EmbeddingFunctionNotFound {
                    name,
                    reason,
                    location: std::panic::Location::caller().to_snafu_location(),
                }
            }
            lancedb::Error::Arrow { source } => Self::Arrow {
                message: source.to_string(),
                location: std::panic::Location::caller().to_snafu_location(),
            },
            lancedb::Error::Lance { source } => Self::from(source),
            _ => Self::OtherLanceDB {
                message: source.to_string(),
                location: std::panic::Location::caller().to_snafu_location(),
            },
        }
    }
}

trait ToSnafuLocation {
    fn to_snafu_location(&'static self) -> snafu::Location;
}

impl ToSnafuLocation for std::panic::Location<'static> {
    fn to_snafu_location(&'static self) -> snafu::Location {
        snafu::Location::new(self.file(), self.line(), self.column())
    }
}
