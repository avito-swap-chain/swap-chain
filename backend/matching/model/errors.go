package model

import "errors"

var (
	ErrVertexAlreadyExists = errors.New("vertex already exists")
	ErrVertexNotFound      = errors.New("vertex not found")
	ErrEdgeAlreadyExists   = errors.New("edge already exists")
	ErrEdgeNotFound        = errors.New("edge not found")
)
