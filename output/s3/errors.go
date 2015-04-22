package s3

import (
	"github.com/spacemonkeygo/errors"
	"polydawn.net/repeatr/output"
)

/*
	Raised if S3 credentials are not available.
*/
var S3ConfigurationMissingError *errors.ErrorClass = output.Error.NewClass("OutputS3ConfigurationMissingError")

/*
	Grouping for an error encountered while talking to the S3 API.
*/
var S3Error *errors.ErrorClass = output.Error.NewClass("OutputS3Error")
