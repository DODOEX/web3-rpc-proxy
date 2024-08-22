package constants

const MaxUint = ^uint(0)
const MinUint = 0
const MaxUint64 = ^uint64(0)
const MinUint64 = 0
const MaxUint32 = ^uint32(0)
const MinUint32 = 0
const MaxUint16 = ^uint16(0)
const MinUint16 = 0
const MaxUint8 = ^uint8(0)
const MinUint8 = 0

const MaxInt = int(MaxUint >> 1)
const MinInt = -MaxInt - 1
const MaxInt64 = int(MaxUint64 >> 1)
const MinInt64 = -MaxInt64 - 1
const MaxInt32 = int(MaxUint32 >> 1)
const MinInt32 = -MaxInt32 - 1
const MaxInt16 = int(MaxUint16 >> 1)
const MinInt16 = -MaxInt32 - 1
const MaxInt8 = int(MaxUint8 >> 1)
const MinInt8 = -MaxInt8 - 1
