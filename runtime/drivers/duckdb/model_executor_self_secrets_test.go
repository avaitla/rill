package duckdb

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSQLMayReferenceCloudStorage(t *testing.T) {
	cases := []struct {
		name string
		sqls []string
		want bool
	}{
		{"plain local model", []string{"SELECT range AS id FROM range(100)"}, false},
		{"local table ref", []string{"SELECT * FROM my_model WHERE x = 1"}, false},
		{"empty", []string{"", ""}, false},
		{"s3 path", []string{"SELECT * FROM read_parquet('s3://bucket/file.parquet')"}, true},
		{"s3 path uppercase", []string{"SELECT * FROM 'S3://Bucket/File.parquet'"}, true},
		{"gcs path", []string{"SELECT * FROM read_csv('gcs://bucket/data.csv')"}, true},
		{"gs path", []string{"SELECT * FROM 'gs://bucket/data.csv'"}, true},
		{"azure path", []string{"SELECT * FROM 'azure://container/blob.parquet'"}, true},
		{"abfss path", []string{"SELECT * FROM 'abfss://container@acct.dfs.core.windows.net/x'"}, true},
		{"https url", []string{"SELECT * FROM read_json('https://example.com/data.json')"}, true},
		{"remote ref only in pre_exec", []string{"SELECT 1", "ATTACH 's3://bucket/other.db' AS other"}, true},
		{"scheme-like word without slashes", []string{"SELECT 'gs' AS col, 'azure' AS cloud"}, false},
		{"substring of identifier", []string{"SELECT logs3 FROM t -- s3 in identifier, no scheme"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			require.Equal(t, c.want, sqlMayReferenceCloudStorage(c.sqls...))
		})
	}
}
