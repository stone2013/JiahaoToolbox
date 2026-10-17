# 数据口径与评分

## 指标

CPU：PDH Processor Information 或 Processor 总体/逻辑处理器百分比；系统计时差分为回退。GetSystemTimes 的 kernel 包含 idle，计算为 `(Δkernel + Δuser - Δidle)/(Δkernel + Δuser)`。超过 64 处理器的分组系统没有可靠 PDH 总体时不报告假全机值。频率来自 PDH Processor Frequency，不使用静态型号频率冒充实时值。

内存：`used=totalPhysical-availablePhysical`，统一由 GlobalMemoryStatusEx 读取；内存条铭牌容量使用 CIM 单独展示。不能把操作系统可用量与条子总容量直接比较后声称丢内存。

磁盘：容量占用 `(total-free)/total`；活动率由 PDH 逻辑盘 `% Idle Time` 取补数，读写为 Bytes/sec。这两种百分比不能互换。仅固定本地分区，容量 5 秒刷新，界面清楚标注活动曲线。

网络：MIB_IF_ROW2 64 位 InOctets/OutOctets 差值 ÷ 两次采样实际经过秒数；先做 uint64 差再转浮点。首样本、接口重置、无效时间间隔为空值。按用户选中单网卡显示，不叠加所有虚拟/物理网卡。本地链路正常不代表能访问互联网。累计流量是接口计数器初始化以来，不是工具箱打开以来。

GPU：DXGI SIZE_T 64 位专用显存容量和共享内存上限；PDH GPU Engine 和 GPU Adapter Memory。共享上限不是已用量；CIM AdapterRAM 的 32 位字段不作为大显存容量来源。温度与 SMART 未接入；缺失只表示本版未获得可靠指标，不能据此判定设备不具备该能力或已经损坏。

默认 IEC 显示 KiB/MiB/GiB，以 1024 为进位；SI 显示 kB/MB/GB，以 1000 为进位；速率统一带 `/s`，基础单位 B/s。bytes 与 bits 不混淆。

## 评分规则

五项权重：系统信息 10、固定磁盘空间 30、内存状态 25、网卡链路 15、硬件信息获取 20。

正常：0 扣分；提醒：该项权重 ×25%；警告：该项权重 ×60%。**各项扣分先用 math.Round 四舍五入为整数**。检测失败/不支持不计入有效分母与覆盖率，不按正常对待。

至少有 4 项可靠检查、有效权重覆盖率≥80%，且磁盘与内存检查有效，才给出分数。`score=round(100×(有效权重-扣分)/有效权重)`。其他情况显示“暂无评分”。合法 100 分只可能由当前可靠检查得出，不是硬编码；即使有分数也必须同时显示覆盖率和失败项。

阈值：分区剩余比例 <15% 提醒、<5% 警告；内存使用率≥85% 提醒、≥95% 警告。内存/磁盘检查拒绝过旧样本、非法总量、范围错误和无效百分比。网卡只检查本地启用/有 IP。硬件检查核对 CPU、GPU、主板、内存条、磁盘 5 类信息是否读取成功，完整正常、≥3 类提醒、少于 3 类检测失败。

“系统信息正常”仅表示成功读取版本信息，不验证支持生命周期或补丁合规。该分数不是硬件健康、剩余寿命、安全风险、病毒检测或系统性能总分，不因为单一高负载宣称硬件损坏。

## 主要技术来源

Microsoft GetSystemTimes：https://learn.microsoft.com/en-us/windows/win32/api/processthreadsapi/nf-processthreadsapi-getsystemtimes
Microsoft GlobalMemoryStatusEx：https://learn.microsoft.com/en-us/windows/win32/api/sysinfoapi/nf-sysinfoapi-globalmemorystatusex
Microsoft MIB_IF_ROW2：https://learn.microsoft.com/en-us/windows/win32/api/netioapi/ns-netioapi-mib_if_row2
Microsoft PDH：https://learn.microsoft.com/en-us/windows/win32/perfctrs/performance-counters-portal
Microsoft DXGI_ADAPTER_DESC1：https://learn.microsoft.com/en-us/windows/win32/api/dxgi/ns-dxgi-dxgi_adapter_desc1
Microsoft GPU 性能口径：https://devblogs.microsoft.com/directx/gpus-in-the-task-manager/
