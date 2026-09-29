package errors

import (
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// InvalidArgument 表示客户端指定了无效的参数。
// 注意：此错误码与 FailedPrecondition 不同。InvalidArgument 表示无论系统状态如何，
// 参数都有问题（如格式错误的文件名）。
// 此错误码不会由 gRPC 框架生成。
//
// 使用场景：参数格式错误或值无效时返回，如地址格式不正确、金额为负数、
// 缺少必需参数等。
func InvalidArgument(parameter string) error {
	return status.Error(codes.InvalidArgument, fmt.Sprintf("invalid %s", parameter))
}

func InvalidArgumentErr(err error) error {
	return status.Error(codes.InvalidArgument, err.Error())
}

func InvalidArgumentf(format string, a ...any) error {
	return status.Errorf(codes.InvalidArgument, format, a...)
}

// Internal 表示内部错误。意味着底层系统的某些不变量被破坏了。
// 如果看到这个错误，说明代码有严重问题。
// 此错误码会在多种内部错误条件下由 gRPC 框架生成。
//
// 使用场景：服务器内部发生未预期的错误时返回，如数据库连接失败、
// 序列化/反序列化错误、系统不变量被破坏等。此错误应谨慎使用，
// 通常表示服务端代码有 bug。
func Internal() error {
	return status.Error(codes.Internal, "")
}

// NotFound 表示请求的实体（例如文件或目录）未找到。
//
// 使用场景：客户端请求的资源不存在时返回，如查询不存在的账户、
// 交易、密钥等。
func NotFound(msg string) error {
	return status.Error(codes.NotFound, msg)
}

func NotFoundf(format string, a ...any) error {
	return status.Errorf(codes.NotFound, format, a...)
}

// AlreadyExists 表示创建实体的尝试失败，因为该实体已存在。
//
// 使用场景：尝试创建已存在的资源时返回，如创建重复的账户、
// 密钥、交易等。
func AlreadyExists(msg string) error {
	return status.Error(codes.AlreadyExists, msg)
}

func AlreadyExistsf(format string, a ...any) error {
	return status.Errorf(codes.AlreadyExists, format, a...)
}

// PermissionDenied 表示调用者没有执行指定操作的权限。
// 注意：此错误码不应用于因耗尽某些资源而导致的拒绝，
// 也不应用于无法识别调用者的情况（应使用 Unauthenticated）。
//
// 使用场景：用户无权访问特定资源或操作时返回，如尝试操作
// 不属于自己的账户、无权签名交易等。
func PermissionDenied(msg string) error {
	return status.Error(codes.PermissionDenied, msg)
}

func PermissionDeniedf(format string, a ...any) error {
	return status.Errorf(codes.PermissionDenied, format, a...)
}

// FailedPrecondition 表示操作被拒绝，因为系统未处于执行操作所需的状态。
// 例如：要删除的目录可能非空、rmdir 操作应用于非目录等。
//
// 选择 FailedPrecondition、Aborted、Unavailable 的判断标准：
//   - 如果客户端可以重试失败的调用，使用 Unavailable
//   - 如果客户端应该在更高层级重试（如重新开始读-改-写序列），使用 Aborted
//   - 如果客户端不应该重试，必须先修复系统状态，使用 FailedPrecondition
//   - 当客户端对资源执行条件性的 GET/UPDATE/DELETE 操作，
//     但服务器上的资源与条件不匹配时，使用 FailedPrecondition
//
// 使用场景：操作的前置条件不满足时返回，如在非空目录上执行删除操作、
// 对不满足条件的资源执行操作等。
func FailedPrecondition(msg string) error {
	return status.Error(codes.FailedPrecondition, msg)
}

func FailedPreconditionf(format string, a ...any) error {
	return status.Errorf(codes.FailedPrecondition, format, a...)
}

// OutOfRange 表示操作尝试超出了有效范围。
// 例如： seek 或读取超过文件末尾。
//
// 与 InvalidArgument 的区别：InvalidArgument 表示无论系统状态如何，
// 参数都有问题（如格式错误的文件名）；而 OutOfRange 表示如果系统状态
// 改变，问题可能得到解决（如 32 位文件系统请求读取超出 [0,2^32-1] 范围的偏移）。
//
// 使用场景：读取或操作超出有效范围时返回，如读取超过文件末尾、
// 分页请求的页码超出范围等。
func OutOfRange(msg string) error {
	return status.Error(codes.OutOfRange, msg)
}

func OutOfRangef(format string, a ...any) error {
	return status.Errorf(codes.OutOfRange, format, a...)
}

// Unimplemented 表示该操作尚未实现或未启用。
//
// 使用场景：请求的功能尚未实现时返回，如调用未实现的 gRPC 方法、
// 使用不支持的链类型或操作等。
func Unimplemented(msg string) error {
	return status.Error(codes.Unimplemented, msg)
}

func Unimplementedf(format string, a ...any) error {
	return status.Errorf(codes.Unimplemented, format, a...)
}

// Unavailable 表示服务当前不可用。
// 这很可能是一个临时状态，通过重试和退避可以纠正。
// 注意：重试非幂等操作可能不安全。
//
// 使用场景：服务暂时不可用时返回，如节点宕机、维护中、网络连接失败等。
func Unavailable(msg string) error {
	return status.Error(codes.Unavailable, msg)
}

func Unavailablef(format string, a ...any) error {
	return status.Errorf(codes.Unavailable, format, a...)
}

// Unauthenticated 表示请求没有有效的认证凭据。
//
// 使用场景：认证失败或未提供认证信息时返回，如 Token 无效、
// 签名验证失败、缺少必要的认证头等。
func Unauthenticated(msg string) error {
	return status.Error(codes.Unauthenticated, msg)
}

func Unauthenticatedf(format string, a ...any) error {
	return status.Errorf(codes.Unauthenticated, format, a...)
}
