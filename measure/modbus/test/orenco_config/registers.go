package main
// Orenco registers scraped from web interface
// Formula: Modbus base = (PointNum * 2) + 999 for float32
//          Modbus base = 40000 + PointNum for int/digital

var orencoRegisters = []struct {
	pointNum int
	name     string
	baseF32  uint16  // float32: pointNum*2 + 999
	baseInt  uint16  // int/digital: 40000 + pointNum
	ptype    string  // A=analog, D=digital, L=label, M=?, T=time
	unit     string
}{
	{1, "RT_AlarmStatus", 1001, 40001, "L", ""}, // RT AlarmStatus
	{2, "RT_PumpMode", 1003, 40002, "L", ""}, // RT PumpMode
	{4, "DT_AlarmStatus", 1007, 40004, "L", ""}, // DT AlarmStatus
	{5, "DT_PumpMode", 1009, 40005, "L", ""}, // DT PumpMode
	{8, "LeadValve", 1015, 40008, "L", ""}, // LeadValve
	{10, "PowerFail", 1019, 40010, "L", ""}, // PowerFail
	{81, "RT_AlarmStatus", 1161, 40081, "L", ""}, // RT AlarmStatus
	{82, "RT_PumpMode", 1163, 40082, "L", ""}, // RT PumpMode
	{83, "RT_TimerMode", 1165, 40083, "L", ""}, // RT TimerMode
	{84, "RT_LeadPump", 1167, 40084, "L", ""}, // RT LeadPump
	{85, "RT_TimerType", 1169, 40085, "L", ""}, // RT TimerType
	{86, "RT_OffTimeStat", 1171, 40086, "L", ""}, // RT OffTimeStat
	{88, "RT_Pump1_Status", 1175, 40088, "L", ""}, // RT Pump1 Status
	{89, "RT_Pump2_Status", 1177, 40089, "L", ""}, // RT Pump2 Status
	{91, "RT_Pump1_Amps", 1181, 40091, "A", "Amps"}, // RT Pump1 Amps
	{92, "RT_Pump2_Amps", 1183, 40092, "A", "Amps"}, // RT Pump2 Amps
	{95, "RT_ActOffTime", 1189, 40095, "A", "Min"}, // RT ActOffTime
	{96, "RT_ActOnTime", 1191, 40096, "A", "Min"}, // RT ActOnTime
	{97, "RT_UseTrndData?", 1193, 40097, "D", "O/F"}, // RT UseTrndData?
	{98, "RT_RetRcrcRatio", 1195, 40098, "A", "x:1"}, // RT RetRcrcRatio
	{99, "RT_MaxOffTime", 1197, 40099, "A", "Min"}, // RT MaxOffTime
	{100, "RT_MinOffTime", 1199, 40100, "A", "Min"}, // RT MinOffTime
	{101, "RT_NoDays_AVG", 1201, 40101, "A", "1-28"}, // RT NoDays-AVG
	{102, "RT_EstAvDalyFlo", 1203, 40102, "A", "GPD"}, // RT EstAvDalyFlo
	{103, "RT_EstPeakDaFlo", 1205, 40103, "A", "GPD"}, // RT EstPeakDaFlo
	{105, "RTAvgDailyFlow", 1209, 40105, "A", "GPD"}, // RTAvgDailyFlow
	{106, "RT_QPeak_Flow", 1211, 40106, "A", "GPD"}, // RT QPeak Flow
	{107, "RTTrendOffTime", 1213, 40107, "A", "Min"}, // RTTrendOffTime
	{108, "RTTrend_OvrOff", 1215, 40108, "A", "Min"}, // RTTrend OvrOff
	{111, "RT_EstFlowOffTm", 1221, 40111, "A", "Min"}, // RT EstFlowOffTm
	{112, "RT_EstFloOvrOff", 1223, 40112, "A", "Min"}, // RT EstFloOvrOff
	{113, "RT_ManualTimSet", 1225, 40113, "D", "O/F"}, // RT ManualTimSet
	{114, "RT_OffTime", 1227, 40114, "A", "Min"}, // RT OffTime
	{115, "RT_OnTime", 1229, 40115, "A", "Min"}, // RT OnTime
	{116, "RT_OvrOffTime", 1231, 40116, "A", "Min"}, // RT OvrOffTime
	{117, "RT_OvrOnTime", 1233, 40117, "A", "Min"}, // RT OvrOnTime
	{118, "RT_HLA_Delay", 1235, 40118, "A", "Sec"}, // RT HLA Delay
	{119, "RT_LagEnable", 1237, 40119, "D", "O/F"}, // RT LagEnable
	{120, "RT_HiAmpAlarm", 1239, 40120, "A", "Amps"}, // RT HiAmpAlarm
	{121, "RT_LoAmpAlarm", 1241, 40121, "A", "Amps"}, // RT LoAmpAlarm
	{122, "RT_Pump1_GPM", 1243, 40122, "A", "GPM"}, // RT Pump1 GPM
	{123, "RT_Pump2_GPM", 1245, 40123, "A", "GPM"}, // RT Pump2 GPM
	{129, "RT_P1CountTday", 1257, 40129, "A", ""}, // RT P1CountTday
	{130, "RT_P2CountTday", 1259, 40130, "A", ""}, // RT P2CountTday
	{132, "RT_P1_TimeTday", 1263, 40132, "A", "Min"}, // RT P1 TimeTday
	{133, "RT_P2_TimeTday", 1265, 40133, "A", "Min"}, // RT P2 TimeTday
	{135, "RT_P1_FlowTday", 1269, 40135, "A", "Gals"}, // RT P1 FlowTday
	{136, "RT_P2_FlowTday", 1271, 40136, "A", "Gals"}, // RT P2 FlowTday
	{142, "RT_TotalCount", 1283, 40142, "A", ""}, // RT TotalCount
	{143, "RT_TotalTime", 1285, 40143, "A", "Min"}, // RT TotalTime
	{144, "RT_TotalFlow", 1287, 40144, "A", "Gals"}, // RT TotalFlow
	{145, "RTEstAvFloRate", 1289, 40145, "A", "GPM"}, // RTEstAvFloRate
	{146, "RTPumpsPerDose", 1291, 40146, "A", ""}, // RTPumpsPerDose
	{148, "RTFloLogColumn", 1295, 40148, "A", ""}, // RTFloLogColumn
	{149, "RTFloLogNumber", 1297, 40149, "A", ""}, // RTFloLogNumber
	{151, "RT_ModeOffTime", 1301, 40151, "A", "Min"}, // RT ModeOffTime
	{152, "RT_ModeOvrOff", 1303, 40152, "A", "Min"}, // RT ModeOvrOff
	{177, "DT_AlarmStatus", 1353, 40177, "L", ""}, // DT AlarmStatus
	{178, "DT_PumpMode", 1355, 40178, "L", ""}, // DT PumpMode
	{179, "DT_TimerMode", 1357, 40179, "L", ""}, // DT TimerMode
	{180, "DT_LeadPump", 1359, 40180, "L", ""}, // DT LeadPump
	{182, "DT_Pump3_Status", 1363, 40182, "L", ""}, // DT Pump3 Status
	{183, "DT_Pump4_Status", 1365, 40183, "L", ""}, // DT Pump4 Status
	{185, "DT_Pump3_Amps", 1369, 40185, "A", "Amps"}, // DT Pump3 Amps
	{186, "DT_Pump4_Amps", 1371, 40186, "A", "Amps"}, // DT Pump4 Amps
	{191, "DT_ActOffTime", 1381, 40191, "A", ""}, // DT ActOffTime
	{192, "DT_ActOnTime", 1383, 40192, "A", ""}, // DT ActOnTime
	{193, "DT_HLA_Delay", 1385, 40193, "A", "Sec"}, // DT HLA Delay
	{195, "DT_PmpHiAmpAlm", 1389, 40195, "A", "Amps"}, // DT PmpHiAmpAlm
	{196, "DT_PmpLoAmpAlm", 1391, 40196, "A", "Amps"}, // DT PmpLoAmpAlm
	{198, "DT_Pump3_GPM", 1395, 40198, "A", "GPM"}, // DT Pump3 GPM
	{199, "DT_Pump4_GPM", 1397, 40199, "A", "GPM"}, // DT Pump4 GPM
	{209, "DT_P3_CountTday", 1417, 40209, "A", ""}, // DT P3 CountTday
	{210, "DT_P4_CountTday", 1419, 40210, "A", ""}, // DT P4 CountTday
	{212, "DT_P3_TimeTday", 1423, 40212, "A", "Min"}, // DT P3 TimeTday
	{213, "DT_P4_TimeTday", 1425, 40213, "A", "Min"}, // DT P4 TimeTday
	{215, "DT_P3_FlowTday", 1429, 40215, "A", "Gal"}, // DT P3 FlowTday
	{216, "DT_P4_FlowTday", 1431, 40216, "A", "Gal"}, // DT P4 FlowTday
	{222, "DT_TotalCount", 1443, 40222, "A", ""}, // DT TotalCount
	{223, "DT_TotalTime", 1445, 40223, "A", "Min"}, // DT TotalTime
	{224, "DT_TotalFlow", 1447, 40224, "A", "Gal"}, // DT TotalFlow
	{241, "LeadZone", 1481, 40241, "L", ""}, // LeadZone
	{243, "Zn1_ValveStatus", 1485, 40243, "D", "O/F"}, // Zn1 ValveStatus
	{244, "Zn2_ValveStatus", 1487, 40244, "D", "O/F"}, // Zn2 ValveStatus
	{245, "Zn3_ValveStatus", 1489, 40245, "D", "O/F"}, // Zn3 ValveStatus
	{248, "Zone1FlowNow", 1495, 40248, "A", "GPM"}, // Zone1FlowNow
	{249, "Zone2FlowNow", 1497, 40249, "A", "GPM"}, // Zone2FlowNow
	{250, "Zone3FlowNow", 1499, 40250, "A", "GPM"}, // Zone3FlowNow
	{253, "Zn1_Available", 1505, 40253, "D", ""}, // Zn1 Available
	{254, "Zn2_Available", 1507, 40254, "D", ""}, // Zn2 Available
	{255, "Zn3_Available", 1509, 40255, "D", ""}, // Zn3 Available
	{256, "NumAvailableZones", 1511, 40256, "A", ""}, // #AvailableZones
	{257, "Zone_1_Enable", 1513, 40257, "D", "O/F"}, // Zone 1 Enable
	{258, "Zone_2_Enable", 1515, 40258, "D", "O/F"}, // Zone 2 Enable
	{259, "Zone_3_Enable", 1517, 40259, "D", "O/F"}, // Zone 3 Enable
	{261, "DT_OffTime", 1521, 40261, "A", "Min"}, // DT OffTime
	{262, "DT_OvrOffTime", 1523, 40262, "A", "Min"}, // DT OvrOffTime
	{263, "Zn1_OnTime", 1525, 40263, "A", "Min"}, // Zn1 OnTime
	{264, "Zn1_OvrOnTime", 1527, 40264, "A", "Min"}, // Zn1 OvrOnTime
	{265, "Zn2_OnTime", 1529, 40265, "A", "Min"}, // Zn2 OnTime
	{266, "Zn2_OvrOnTime", 1531, 40266, "A", "Min"}, // Zn2 OvrOnTime
	{267, "Zn3_OnTime", 1533, 40267, "A", "Min"}, // Zn3 OnTime
	{268, "Zn3_OvrOnTime", 1535, 40268, "A", "Min"}, // Zn3 OvrOnTime
	{270, "Zn1MaxFlow", 1539, 40270, "A", "Gal"}, // Zn1MaxFlow
	{271, "Zn2MaxFlow", 1541, 40271, "A", "Gal"}, // Zn2MaxFlow
	{272, "Zn3MaxFlow", 1543, 40272, "A", "Gal"}, // Zn3MaxFlow
	{273, "Zone1_CountTday", 1545, 40273, "A", ""}, // Zone1 CountTday
	{274, "Zone2_CountTday", 1547, 40274, "A", ""}, // Zone2 CountTday
	{275, "Zone3_CountTday", 1549, 40275, "A", ""}, // Zone3 CountTday
	{277, "Zone1_TimeTday", 1553, 40277, "A", "Min"}, // Zone1 TimeTday
	{278, "Zone2_TimeTday", 1555, 40278, "A", "Min"}, // Zone2 TimeTday
	{279, "Zone3_TimeTday", 1557, 40279, "A", "Min"}, // Zone3 TimeTday
	{281, "Zone1FlowTdy", 1561, 40281, "A", "Gal"}, // Zone1FlowTdy
	{282, "Zone2FlowTdy", 1563, 40282, "A", "Gal"}, // Zone2FlowTdy
	{283, "Zone3FlowTdy", 1565, 40283, "A", "Gal"}, // Zone3FlowTdy
	{385, "RT_HighLevel", 1769, 40385, "D", ""}, // RT HighLevel
	{386, "RT_LowLevel", 1771, 40386, "D", ""}, // RT LowLevel
	{387, "RT_P1_HighAmps", 1773, 40387, "D", ""}, // RT P1 HighAmps
	{388, "RT_P1_LowAmps", 1775, 40388, "D", ""}, // RT P1 LowAmps
	{389, "RT_P2_HighAmps", 1777, 40389, "D", ""}, // RT P2 HighAmps
	{390, "RT_P2_LowAmps", 1779, 40390, "D", ""}, // RT P2 LowAmps
	{392, "DT_HighLevel", 1783, 40392, "D", ""}, // DT HighLevel
	{393, "DT_LowLevel", 1785, 40393, "D", ""}, // DT LowLevel
	{394, "DT_P3_HighAmps", 1787, 40394, "D", ""}, // DT P3 HighAmps
	{395, "DT_P3_LowAmps", 1789, 40395, "D", ""}, // DT P3 LowAmps
	{396, "DT_P4_HighAmps", 1791, 40396, "D", ""}, // DT P4 HighAmps
	{397, "DT_P4_LowAmps", 1793, 40397, "D", ""}, // DT P4 LowAmps
	{399, "PowerFail", 1797, 40399, "D", ""}, // PowerFail
	{417, "RT_AltEnable", 1833, 40417, "D", ""}, // RT AltEnable
	{418, "RT_AltSignal", 1835, 40418, "D", ""}, // RT AltSignal
	{420, "DT_AltEnable", 1839, 40420, "D", ""}, // DT AltEnable
	{421, "DT_AltSignal", 1841, 40421, "D", ""}, // DT AltSignal
	{433, "GeneralAlarm", 1865, 40433, "D", ""}, // GeneralAlarm
	{434, "AudAlarmStart", 1867, 40434, "D", ""}, // AudAlarmStart
	{435, "AlarmSilence", 1869, 40435, "D", ""}, // AlarmSilence
	{449, "AudibleDelay", 1897, 40449, "A", "Min"}, // AudibleDelay
	{450, "AudibleReact", 1899, 40450, "A", "Min"}, // AudibleReact
	{451, "PageInterval", 1901, 40451, "A", "Min"}, // PageInterval
	{452, "AmpAlarmDelay", 1903, 40452, "A", "Sec"}, // AmpAlarmDelay
	{461, "MBNum1_Activate", 1921, 40461, "D", ""}, // MB#1 Activate
	{462, "MBNum2_Activate", 1923, 40462, "D", ""}, // MB#2 Activate
	{463, "MBNum3_Activate", 1925, 40463, "D", ""}, // MB#3 Activate
	{464, "MBNum4_Activate", 1927, 40464, "D", ""}, // MB#4 Activate
	{465, "RT_P1_AvAmpTday", 1929, 40465, "A", "Amps"}, // RT P1 AvAmpTday
	{466, "RT_P2_AvAmpTday", 1931, 40466, "A", "Amps"}, // RT P2 AvAmpTday
	{467, "DT_P3_AvAmpTday", 1933, 40467, "A", "Amps"}, // DT P3 AvAmpTday
	{468, "DT_P4_AvAmpTday", 1935, 40468, "A", "Amps"}, // DT P4 AvAmpTday
	{481, "RT_P1_Reset", 1961, 40481, "D", ""}, // RT P1 Reset
	{482, "RT_P2_Reset", 1963, 40482, "D", ""}, // RT P2 Reset
	{483, "DT_P3_Reset", 1965, 40483, "D", ""}, // DT P3 Reset
	{484, "DT_P4_Reset", 1967, 40484, "D", ""}, // DT P4 Reset
	{485, "Zone1_Reset", 1969, 40485, "D", ""}, // Zone1 Reset
	{486, "Zone2_Reset", 1971, 40486, "D", ""}, // Zone2 Reset
	{487, "Zone3_Reset", 1973, 40487, "D", ""}, // Zone3 Reset
	{497, "RT_P1_CT_Delay", 1993, 40497, "D", ""}, // RT P1 CT Delay
	{498, "RT_P2_CT_Delay", 1995, 40498, "D", ""}, // RT P2 CT Delay
	{499, "DT_P3_CT_Delay", 1997, 40499, "D", ""}, // DT P3 CT Delay
	{500, "DT_P4_CT_Delay", 1999, 40500, "D", ""}, // DT P4 CT Delay
	{561, "RT_HLA_Lag", 2121, 40561, "D", ""}, // RT HLA/Lag
	{562, "RT_OvrOn_Off", 2123, 40562, "D", ""}, // RT OvrOn/Off
	{563, "RT_RO_LLA", 2125, 40563, "D", ""}, // RT RO/LLA
	{565, "DT_HLA", 2129, 40565, "D", ""}, // DT HLA
	{566, "DT_Lag_Enable", 2131, 40566, "D", ""}, // DT Lag Enable
	{567, "DT_OvrOn_Off", 2133, 40567, "D", ""}, // DT OvrOn/Off
	{568, "DT_TmrOn_Off", 2135, 40568, "D", ""}, // DT TmrOn/Off
	{569, "DT_RO_LLA", 2137, 40569, "D", ""}, // DT RO/LLA
	{574, "PowerFail", 2147, 40574, "D", ""}, // PowerFail
	{576, "PushToSilence", 2151, 40576, "D", ""}, // PushToSilence
	{577, "SpareDI1", 2153, 40577, "D", ""}, // SpareDI1
	{578, "SpareDI2", 2155, 40578, "D", ""}, // SpareDI2
	{593, "RT_Pump1_CT", 2185, 40593, "A", "Amps"}, // RT Pump1 CT
	{594, "RT_Pump2_CT", 2187, 40594, "A", "Amps"}, // RT Pump2 CT
	{595, "DT_Pump3_CT", 2189, 40595, "A", "Amps"}, // DT Pump3 CT
	{596, "DT_Pump4_CT", 2191, 40596, "A", "Amps"}, // DT Pump4 CT
	{598, "SpareAI1", 2195, 40598, "A", "Volt"}, // SpareAI1
	{599, "SpareAI2", 2197, 40599, "A", "Volt"}, // SpareAI2
	{609, "RT_Pump1", 2217, 40609, "D", ""}, // RT Pump1
	{610, "RT_Pump2", 2219, 40610, "D", ""}, // RT Pump2
	{611, "DT_Pump3", 2221, 40611, "D", ""}, // DT Pump3
	{612, "DT_Pump4", 2223, 40612, "D", ""}, // DT Pump4
	{614, "Zone1_Valve", 2227, 40614, "D", ""}, // Zone1 Valve
	{615, "Zone2_Valve", 2229, 40615, "D", ""}, // Zone2 Valve
	{616, "Zone3_Valve", 2231, 40616, "D", ""}, // Zone3 Valve
	{620, "SpareDO", 2239, 40620, "D", ""}, // SpareDO
	{623, "AlarmLight", 2245, 40623, "D", ""}, // AlarmLight
	{624, "AudibleAlarm", 2247, 40624, "D", ""}, // AudibleAlarm
}
