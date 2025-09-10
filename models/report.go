package models

import "time"

type ReportVersion struct {
	Version           string
	Label             string
	ConfigVersion     string
	User              UserMapping
	CreationDate      time.Time
	CreationTime      time.Time
	Comment           string
	AddedCount        int
	ChangedCount      int
	FileName          string
	StoragePath       string
	Storage Storage
	Extension Extension
}

type Report struct {
	StoragePath string
	ReportDate  time.Time
	ReportTime  time.Time
	Versions    []ReportVersion
	FileName    string
}
