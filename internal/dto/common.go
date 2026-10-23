// Package dto berisi struktur data untuk request/response API.
//
// Tag JSON memakai camelCase agar cocok langsung dengan tipe TypeScript di
// front-end bumn-academy.
package dto

import "github.com/bumn-academy/be-bumn-academy/internal/pkg/response"

// Pagination adalah parameter paginasi standar.
type Pagination struct {
	Page    int `form:"page" json:"page"`
	PerPage int `form:"perPage" json:"perPage"`
}

// Normalize membatasi nilai paginasi agar selalu masuk akal.
func (p *Pagination) Normalize() {
	if p.Page < 1 {
		p.Page = 1
	}
	if p.PerPage < 1 {
		p.PerPage = 20
	}
	if p.PerPage > 100 {
		p.PerPage = 100
	}
}

// Offset menghitung offset SQL.
func (p Pagination) Offset() int { return (p.Page - 1) * p.PerPage }

// Limit mengembalikan batas jumlah baris.
func (p Pagination) Limit() int { return p.PerPage }

// Meta membentuk metadata paginasi.
func (p Pagination) Meta(total int64) response.Meta {
	meta := response.Meta{Page: p.Page, PerPage: p.PerPage, Total: total}
	if p.PerPage > 0 {
		meta.TotalPages = int((total + int64(p.PerPage) - 1) / int64(p.PerPage))
	}
	return meta
}
