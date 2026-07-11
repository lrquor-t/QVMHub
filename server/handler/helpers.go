package handler

// helpers.go 预留给 controller-native 跨 handler 公共工具。
// Task 5 清理：原先存放的 VM 业务校验/构建辅助（respondVMListError、
// buildVMListOptions、validateVMNameNotExists、validateSwitchBridges、
// sanitizeUserMemoryDynamicRequest 等）仅被已删除的 vm.go/clone.go/user_self.go 等
// 业务 handler 调用，controller-native handler 无任何引用，已整体移除以解除对
// service/vm/memory、service.DomainExistsRPC 等本地执行依赖的耦合。
