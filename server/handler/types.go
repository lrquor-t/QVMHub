package handler

// types.go 预留给 controller-native 请求/响应 DTO。
// Task 5 清理：原先存放的 VM 业务请求体（VmOperateRequest/VmEditRequest/
// RescueVmRequest/ResetLinuxPasswordRequest/VmAddDiskItem）随业务 handler 一并移除，
// 这些类型仅被已删除的 vm.go/vm_rescue.go 等引用，controller 不再需要。
