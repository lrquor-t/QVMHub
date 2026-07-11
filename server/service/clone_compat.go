package service

// Clone compatibility types - delegate to service/clone subpackage
import clonepkg "qvmhub/service/clone"

type CloneParams = clonepkg.CloneParams
type BatchCloneParams = clonepkg.BatchCloneParams
type ReinstallParams = clonepkg.ReinstallParams
type CloneResult = clonepkg.CloneResult
type LinkedCloneParams = clonepkg.LinkedCloneParams
type LinkedCloneResult = clonepkg.LinkedCloneResult