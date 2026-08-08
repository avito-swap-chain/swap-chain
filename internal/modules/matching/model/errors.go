package model

import "errors"

var ErrVertexAlreadyExists = errors.New("vertex already exists")
var ErrVertexNotFound = errors.New("vertex not found")
var ErrEdgeAlreadyExists = errors.New("edge already exists")
var ErrEdgeNotFound = errors.New("edge not found")
