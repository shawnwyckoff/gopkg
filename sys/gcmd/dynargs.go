package gcmd

// 给运行中的进程以命令行方式传参，参数的发送和获取都仅限于通过此模块进行，不支持其他不使用此模块的Go程序以及其他编程语言的程序，其实就是用chan进行进程间通信
