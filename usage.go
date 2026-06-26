package main

var usage = `
es is a CLI for ElasticSearch. https://github.com/arp242/es

Flags:

    -es   ElasticSearch HTTP URL. Default: http://elastic:elastic@127.0.0.1:9200
    -x    Use expanded (vertical) output.
    -v    Show HTTP requests and responses.

Commands:

    es                            List all indexes.
    es «index» [select/list/ls]   Select/list rows for this index.
    es «index» describe           Show index parameters.
    es «index» drop               Delete index.

    The command and index may be reveresed; "es «index» drop" and "es drop
    «index»" both work. The command is optional for the "select" command ("es
    «index»" and "es «index» select" are identical).

Flags for drop:

    The "drop" command accepts multiple indexes separated by commas, as well as
    a globbing pattern. e.g. "es drop 'abc,foo-*'" will drop "abc" and all
    indexes starting with "foo-". A glob pattern must match at least one index.

Flags for select:

    -s, -select      Columns to select; as comma-separated list. "*" selects all
                     rows with at least one non-zero value in the result set,
                     "*.all" selects all rows. Default: "*".

    -l, -limit       Limit number of rows to fetch. Default: 100

    -o, -order       Sort order; may optionally be followed by :asc or :desc.
                     Default: not specified.

    -w, -where       Query string to search, as «field»:«text». If «field» is
                     omitted it will search all fields. Supports "AND" and "OR"
                     (case-sensitive); grouping with ( ) and quoting with " ".

                     Remember shell quoting; e.g. use -w '"two words"'.

                     Examples:

                        a OR b
                        col:a
                        col:"two words"
                        col:(a OR b)
                        col:(a OR b) AND other:c

                     For full documentation, see:
                     https://www.elastic.co/guide/en/elasticsearch/reference/8.19/query-dsl-query-string-query.html#query-string-syntax
`[1:]
