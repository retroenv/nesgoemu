package rainbow

// Official register symbols: https://github.com/BrokeStudio/rainbow-net/blob/master/NES/mapper-registers.s

const (
	registerStart = regPRGControl
	registerEnd   = 0x42FF

	regPRGControl         = 0x4100
	regLowBankUpperStart  = 0x4106
	regLowBankUpperEnd    = 0x4107
	regHighBankUpperStart = 0x4108
	regHighBankUpperEnd   = 0x410F
	regFPGABankSelect     = 0x4115
	regLowBankLowerStart  = 0x4116
	regLowBankLowerEnd    = 0x4117
	regHighBankLowerStart = 0x4118
	regHighBankLowerEnd   = 0x411F

	regCHRControl        = 0x4120
	regBGExtModeOffset   = 0x4121
	regFillTile          = 0x4124
	regFillAttribute     = 0x4125
	regNTBankStart       = 0x4126
	regNTBankEnd         = 0x4129
	regNTControlStart    = 0x412A
	regNTControlEnd      = 0x412D
	regNTWindowBank      = 0x412E
	regNTWindowControl   = 0x412F
	regCHRBankUpperStart = 0x4130
	regCHRBankUpperEnd   = 0x413F
	regCHRBankLowerStart = 0x4140
	regCHRBankLowerEnd   = 0x414F

	regScanIRQLatch         = 0x4150
	regScanIRQControl       = 0x4151
	regScanIRQAcknowledge   = 0x4152
	regScanIRQOffset        = 0x4153
	regScanIRQJitter        = 0x4154
	regCycleIRQParity       = 0x4157
	regCycleIRQReloadUpper  = 0x4158
	regCycleIRQReloadLower  = 0x4159
	regCycleIRQControl      = 0x415A
	regCycleIRQAcknowledge  = 0x415B
	regFPGAAutoAddressUpper = 0x415C
	regFPGAAutoAddressLower = 0x415D
	regFPGAAutoIncrement    = 0x415E
	regFPGAAutoData         = 0x415F

	regPlatformVersion = 0x4160
	regIRQStatus       = 0x4161
	regVectorControl   = 0x416B
	regNMIVectorUpper  = 0x416C
	regNMIVectorLower  = 0x416D
	regIRQVectorUpper  = 0x416E
	regIRQVectorLower  = 0x416F

	regWindowSplitStart = 0x4170
	regWindowSplitEnd   = 0x4175

	regESPControl      = 0x4190
	regESPStatus       = 0x4191
	regESPStart        = 0x4192
	regESPReceivePage  = 0x4193
	regESPTransmitPage = 0x4194

	regSpriteBankLowerStart = 0x4200
	regSpriteBankLowerEnd   = 0x423F
	regSpriteBankUpper      = 0x4240
	regOAMSlowPage          = 0x4241
	regOAMExtendedPage      = 0x4242
	regOAMLimit             = 0x4243

	oamRoutineStart       = 0x4280
	oamSpriteRoutineStart = 0x4282
)
