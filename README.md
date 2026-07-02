`es` is a CLI for ElasticSearch.

It's not intended to be a complete management tool; it's a fairly thin frontend
for the HTTP API to avoid dealing with all this annoying JSON. It's some stuff I
found useful over the last few years and capabilities are added on an as-needed
basis.

Usage:

    es                            List all indexes.
    es «index» [select/list/ls]   Select/list rows for this index.
    es «index» delete             Delete rows by query.
    es «index» describe           Show index parameters.
    es «index» drop               Delete index.

See `es help` for full documentation.
