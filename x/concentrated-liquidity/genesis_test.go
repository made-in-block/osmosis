package concentrated_liquidity_test

import (
	"fmt"
	"testing"
	"time"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/stretchr/testify/assert"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/osmosis-labs/osmosis/osmoutils/accum"
	osmoapp "github.com/osmosis-labs/osmosis/v16/app"
	cl "github.com/osmosis-labs/osmosis/v16/x/concentrated-liquidity"
	clmodule "github.com/osmosis-labs/osmosis/v16/x/concentrated-liquidity/clmodule"
	"github.com/osmosis-labs/osmosis/v16/x/concentrated-liquidity/model"
	"github.com/osmosis-labs/osmosis/v16/x/concentrated-liquidity/types"
	"github.com/osmosis-labs/osmosis/v16/x/concentrated-liquidity/types/genesis"
)

type singlePoolGenesisEntry struct {
	pool                    model.Pool
	tick                    []genesis.FullTick
	positionData            []genesis.PositionData
	spreadFactorAccumValues genesis.AccumObject
	incentiveAccumulators   []genesis.AccumObject
	incentiveRecords        []types.IncentiveRecord
}

var (
	baseGenesis = genesis.GenesisState{
		Params: types.Params{
			AuthorizedTickSpacing:        []uint64{1, 10, 100, 1000},
			AuthorizedSpreadFactors:      []sdk.Dec{sdk.MustNewDecFromStr("0.0001"), sdk.MustNewDecFromStr("0.0003"), sdk.MustNewDecFromStr("0.0005")},
			AuthorizedQuoteDenoms:        []string{ETH, USDC},
			BalancerSharesRewardDiscount: types.DefaultBalancerSharesDiscount,
			AuthorizedUptimes:            types.DefaultAuthorizedUptimes,
		},
		PoolData:              []genesis.GenesisPoolData{},
		NextIncentiveRecordId: 2,
		NextPositionId:        3,
	}
	testCoins    = sdk.NewDecCoins(cl.HundredFooCoins)
	testTickInfo = model.TickInfo{
		LiquidityGross: sdk.OneDec(),
		LiquidityNet:   sdk.OneDec(),
		SpreadRewardGrowthOppositeDirectionOfLastTraversal: testCoins,
		UptimeTrackers: model.UptimeTrackers{
			List: []model.UptimeTracker{
				{
					UptimeGrowthOutside: testCoins,
				},
			},
		},
	}
	defaultFullTick = genesis.FullTick{
		TickIndex: 0,
		Info:      testTickInfo,
	}

	defaultFullTickWithoutPoolId = genesis.FullTick{
		TickIndex: 0,
		Info:      testTickInfo,
	}

	testPositionModel = genesis.PositionWithoutPoolId{
		PositionId: 1,
		Address:    testAddressOne.String(),
		Liquidity:  sdk.OneDec(),
		LowerTick:  -1,
		UpperTick:  100,
		JoinTime:   defaultBlockTime,
	}

	testSpreadRewardAccumRecord = accum.Record{
		NumShares:             sdk.OneDec(),
		AccumValuePerShare:    sdk.NewDecCoins(sdk.NewDecCoin("foo", sdk.NewInt(10))),
		UnclaimedRewardsTotal: sdk.NewDecCoins(sdk.NewDecCoin("foo", sdk.NewInt(5))),
		Options:               nil,
	}

	accumRecord = accum.Record{
		NumShares:             sdk.OneDec(),
		AccumValuePerShare:    sdk.NewDecCoins(sdk.NewDecCoin("foo", sdk.NewInt(50))),
		UnclaimedRewardsTotal: sdk.NewDecCoins(sdk.NewDecCoin("foo", sdk.NewInt(25))),
		Options:               nil,
	}

	// five records because we have 5 supported uptimes
	testUptimeAccumRecord = []accum.Record{
		accumRecord,
		accumRecord,
		accumRecord,
		accumRecord,
		accumRecord,
		accumRecord,
	}
)

func accumRecordWithDefinedValues(accumRecord accum.Record, numShares sdk.Dec, initAccumValue, unclaimedRewards sdk.Int) accum.Record {
	accumRecord.NumShares = numShares
	accumRecord.AccumValuePerShare = sdk.NewDecCoins(sdk.NewDecCoin("uion", initAccumValue))
	accumRecord.UnclaimedRewardsTotal = sdk.NewDecCoins(sdk.NewDecCoin("uosmo", unclaimedRewards))
	return accumRecord
}

func withPositionId(position genesis.PositionWithoutPoolId, positionId uint64) *genesis.PositionWithoutPoolId {
	position.PositionId = positionId
	return &position
}

func incentiveAccumsWithPoolId(poolId uint64) []genesis.AccumObject {
	return []genesis.AccumObject{
		{
			Name: types.KeyUptimeAccumulator(poolId, uint64(0)),
			AccumContent: &accum.AccumulatorContent{
				AccumValue:  sdk.NewDecCoins(sdk.NewDecCoin("foo", sdk.NewInt(20))),
				TotalShares: sdk.NewDec(20),
			},
		},
		{
			Name: types.KeyUptimeAccumulator(poolId, uint64(1)),
			AccumContent: &accum.AccumulatorContent{
				AccumValue:  sdk.NewDecCoins(sdk.NewDecCoin("bar", sdk.NewInt(20))),
				TotalShares: sdk.NewDec(30),
			},
		},
		{
			Name: types.KeyUptimeAccumulator(poolId, uint64(2)),
			AccumContent: &accum.AccumulatorContent{
				AccumValue:  sdk.NewDecCoins(sdk.NewDecCoin("baz", sdk.NewInt(10))),
				TotalShares: sdk.NewDec(10),
			},
		},
		{
			Name: types.KeyUptimeAccumulator(poolId, uint64(3)),
			AccumContent: &accum.AccumulatorContent{
				AccumValue:  sdk.NewDecCoins(sdk.NewDecCoin("qux", sdk.NewInt(20))),
				TotalShares: sdk.NewDec(20),
			},
		},
		{
			Name: types.KeyUptimeAccumulator(poolId, uint64(4)),
			AccumContent: &accum.AccumulatorContent{
				AccumValue:  sdk.NewDecCoins(sdk.NewDecCoin("quux", sdk.NewInt(20))),
				TotalShares: sdk.NewDec(20),
			},
		},
		{
			Name: types.KeyUptimeAccumulator(poolId, uint64(5)),
			AccumContent: &accum.AccumulatorContent{
				AccumValue:  sdk.NewDecCoins(sdk.NewDecCoin("quuux", sdk.NewInt(10))),
				TotalShares: sdk.NewDec(20),
			},
		},
	}
}

// setupGenesis initializes the GenesisState with the given poolGenesisEntries data.
// It returns an updated GenesisState after processing the input data.
//
// baseGenesis is the initial GenesisState.
// poolGenesisEntries is a slice of singlePoolGenesisEntry structures, each containing data
// for a single pool (the pool itself, its ticks, positions, incentives records, accumulators and the next position ID).
//
// The function iterates over the poolGenesisEntries, and for each entry, it creates a new Any type using
// the pool's data, then appends a new PoolData structure containing the pool and its corresponding
// ticks to the baseGenesis.PoolData. It also appends the corresponding positions to the
// baseGenesis.Positions, along with the incentive records and accumulator values for spread rewards and incentives.
func setupGenesis(baseGenesis genesis.GenesisState, poolGenesisEntries []singlePoolGenesisEntry) genesis.GenesisState {
	for _, poolGenesisEntry := range poolGenesisEntries {
		poolCopy := poolGenesisEntry.pool
		poolAny, err := codectypes.NewAnyWithValue(&poolCopy)
		if err != nil {
			panic(err)
		}
		baseGenesis.PoolData = append(baseGenesis.PoolData, genesis.GenesisPoolData{
			Pool:                    poolAny,
			PositionData:            poolGenesisEntry.positionData,
			Ticks:                   poolGenesisEntry.tick,
			SpreadRewardAccumulator: poolGenesisEntry.spreadFactorAccumValues,
			IncentivesAccumulators:  poolGenesisEntry.incentiveAccumulators,
			IncentiveRecords:        poolGenesisEntry.incentiveRecords,
		})
		baseGenesis.NextPositionId = uint64(len(poolGenesisEntry.positionData))
	}
	return baseGenesis
}

// TestInitGenesis tests the InitGenesis function of the ConcentratedLiquidityKeeper.
// It checks that the state is initialized correctly based on the provided genesis.
func (s *KeeperTestSuite) TestInitGenesis() {
	s.SetupTest()
	poolE := s.PrepareConcentratedPool()
	poolOne, ok := poolE.(*model.Pool)
	s.Require().True(ok)

	poolE = s.PrepareConcentratedPool()
	poolTwo, ok := poolE.(*model.Pool)
	s.Require().True(ok)

	defaultTime1 := time.Unix(100, 100)
	defaultTime2 := time.Unix(300, 100)

	testCase := []struct {
		name                            string
		genesis                         genesis.GenesisState
		expectedPools                   []model.Pool
		expectedTicksPerPoolId          map[uint64][]genesis.FullTick
		expectedPositionData            []genesis.PositionData
		expectedspreadFactorAccumValues []genesis.AccumObject
		expectedIncentiveRecords        []types.IncentiveRecord
	}{
		{
			name: "one pool, one position, two ticks, one accumulator, two incentive records",
			genesis: setupGenesis(baseGenesis, []singlePoolGenesisEntry{
				{
					pool: *poolOne,
					tick: []genesis.FullTick{
						withTickIndex(defaultFullTick, -10),
						withTickIndex(defaultFullTick, 10),
					},
					positionData: []genesis.PositionData{
						{
							LockId:                  1,
							Position:                &testPositionModel,
							SpreadRewardAccumRecord: testSpreadRewardAccumRecord,
							UptimeAccumRecords:      testUptimeAccumRecord,
						},
						{
							LockId:                  0,
							Position:                withPositionId(testPositionModel, 2),
							SpreadRewardAccumRecord: testSpreadRewardAccumRecord,
							UptimeAccumRecords: []accum.Record{
								accumRecordWithDefinedValues(accumRecord, sdk.NewDec(10000), sdk.NewInt(100), sdk.NewInt(50)),
								accumRecordWithDefinedValues(accumRecord, sdk.NewDec(1000), sdk.NewInt(100), sdk.NewInt(50)),
								accumRecordWithDefinedValues(accumRecord, sdk.NewDec(100), sdk.NewInt(100), sdk.NewInt(50)),
								accumRecordWithDefinedValues(accumRecord, sdk.NewDec(10), sdk.NewInt(100), sdk.NewInt(50)),
								accumRecordWithDefinedValues(accumRecord, sdk.NewDec(1), sdk.NewInt(100), sdk.NewInt(50)),
								accumRecordWithDefinedValues(accumRecord, sdk.NewDec(1), sdk.NewInt(100), sdk.NewInt(50)),
							},
						},
					},
					spreadFactorAccumValues: genesis.AccumObject{
						Name: types.KeySpreadRewardPoolAccumulator(1),
						AccumContent: &accum.AccumulatorContent{
							AccumValue:  sdk.NewDecCoins(sdk.NewDecCoin("foo", sdk.NewInt(10))),
							TotalShares: sdk.NewDec(10),
						},
					},
					incentiveAccumulators: incentiveAccumsWithPoolId(1),
					incentiveRecords: []types.IncentiveRecord{
						{
							PoolId: uint64(1),
							IncentiveRecordBody: types.IncentiveRecordBody{
								RemainingCoin: sdk.NewDecCoin("bar", sdk.NewInt(15)),
								EmissionRate:  sdk.NewDec(20),
								StartTime:     defaultTime2,
							},
							MinUptime:   testUptimeOne,
							IncentiveId: 1,
						},
						{
							PoolId: uint64(1),
							IncentiveRecordBody: types.IncentiveRecordBody{
								RemainingCoin: sdk.NewDecCoin("foo", sdk.NewInt(5)),
								EmissionRate:  sdk.NewDec(10),
								StartTime:     defaultTime1,
							},
							MinUptime:   testUptimeOne,
							IncentiveId: 2,
						},
					},
				},
			}),
			expectedPools: []model.Pool{
				*poolOne,
			},
			expectedTicksPerPoolId: map[uint64][]genesis.FullTick{
				1: {
					withTickIndex(defaultFullTick, -10),
					withTickIndex(defaultFullTick, 10),
				},
			},
			expectedPositionData: []genesis.PositionData{
				{
					LockId:                  1,
					Position:                &testPositionModel,
					SpreadRewardAccumRecord: testSpreadRewardAccumRecord,
					UptimeAccumRecords:      testUptimeAccumRecord,
				},
				{
					LockId:                  0,
					Position:                withPositionId(testPositionModel, 2),
					SpreadRewardAccumRecord: testSpreadRewardAccumRecord,
					UptimeAccumRecords: []accum.Record{
						accumRecordWithDefinedValues(accumRecord, sdk.NewDec(10000), sdk.NewInt(100), sdk.NewInt(50)),
						accumRecordWithDefinedValues(accumRecord, sdk.NewDec(1000), sdk.NewInt(100), sdk.NewInt(50)),
						accumRecordWithDefinedValues(accumRecord, sdk.NewDec(100), sdk.NewInt(100), sdk.NewInt(50)),
						accumRecordWithDefinedValues(accumRecord, sdk.NewDec(10), sdk.NewInt(100), sdk.NewInt(50)),
						accumRecordWithDefinedValues(accumRecord, sdk.NewDec(1), sdk.NewInt(100), sdk.NewInt(50)),
						accumRecordWithDefinedValues(accumRecord, sdk.NewDec(1), sdk.NewInt(100), sdk.NewInt(50)),
					},
				},
			},
			expectedspreadFactorAccumValues: []genesis.AccumObject{
				{
					Name: types.KeySpreadRewardPoolAccumulator(1),
					AccumContent: &accum.AccumulatorContent{
						AccumValue:  sdk.NewDecCoins(sdk.NewDecCoin("foo", sdk.NewInt(10))),
						TotalShares: sdk.NewDec(10),
					},
				},
			},
			expectedIncentiveRecords: []types.IncentiveRecord{
				{
					PoolId: uint64(1),
					IncentiveRecordBody: types.IncentiveRecordBody{
						RemainingCoin: sdk.NewDecCoin("bar", sdk.NewInt(15)),
						EmissionRate:  sdk.NewDec(20),
						StartTime:     defaultTime2,
					},
					MinUptime: testUptimeOne,
				},
				{
					PoolId: uint64(1),
					IncentiveRecordBody: types.IncentiveRecordBody{
						RemainingCoin: sdk.NewDecCoin("foo", sdk.NewInt(5)),
						EmissionRate:  sdk.NewDec(10),
						StartTime:     defaultTime1,
					},
					MinUptime: testUptimeOne,
				},
			},
		},
		{
			name: "two pools, two positions, one tick pool one, two ticks pool two, two accumulators, one incentive records each",
			genesis: setupGenesis(baseGenesis, []singlePoolGenesisEntry{
				{
					pool: *poolOne,
					tick: []genesis.FullTick{
						withTickIndex(defaultFullTick, -1234),
					},
					positionData: []genesis.PositionData{
						{
							LockId:                  1,
							Position:                &testPositionModel,
							SpreadRewardAccumRecord: testSpreadRewardAccumRecord,
							UptimeAccumRecords:      testUptimeAccumRecord,
						},
					},
					spreadFactorAccumValues: genesis.AccumObject{
						Name: types.KeySpreadRewardPoolAccumulator(1),
						AccumContent: &accum.AccumulatorContent{
							AccumValue:  sdk.NewDecCoins(sdk.NewDecCoin("foo", sdk.NewInt(10))),
							TotalShares: sdk.NewDec(10),
						},
					},
					incentiveAccumulators: incentiveAccumsWithPoolId(1),
					incentiveRecords: []types.IncentiveRecord{
						{
							PoolId: uint64(1),
							IncentiveRecordBody: types.IncentiveRecordBody{
								RemainingCoin: sdk.NewDecCoin("foo", sdk.NewInt(5)),
								EmissionRate:  sdk.NewDec(10),
								StartTime:     defaultTime1,
							},
							MinUptime:   testUptimeOne,
							IncentiveId: 1,
						},
					},
				},
				{
					pool: *poolTwo,
					tick: []genesis.FullTick{
						withTickIndex(defaultFullTick, 0),
						withTickIndex(defaultFullTick, 999),
					},
					positionData: []genesis.PositionData{
						{
							LockId:   2,
							Position: withPositionId(testPositionModel, DefaultPositionId+1),
							UptimeAccumRecords: []accum.Record{
								accumRecordWithDefinedValues(accumRecord, sdk.NewDec(99999), sdk.NewInt(10), sdk.NewInt(5)),
								accumRecordWithDefinedValues(accumRecord, sdk.NewDec(9999), sdk.NewInt(10), sdk.NewInt(5)),
								accumRecordWithDefinedValues(accumRecord, sdk.NewDec(999), sdk.NewInt(100), sdk.NewInt(50)),
								accumRecordWithDefinedValues(accumRecord, sdk.NewDec(99), sdk.NewInt(50), sdk.NewInt(25)),
								accumRecordWithDefinedValues(accumRecord, sdk.NewDec(9), sdk.NewInt(50), sdk.NewInt(25)),
								accumRecordWithDefinedValues(accumRecord, sdk.NewDec(9), sdk.NewInt(50), sdk.NewInt(25)),
							},
						},
					},

					spreadFactorAccumValues: genesis.AccumObject{
						Name: types.KeySpreadRewardPoolAccumulator(2),
						AccumContent: &accum.AccumulatorContent{
							AccumValue:  sdk.NewDecCoins(sdk.NewDecCoin("bar", sdk.NewInt(20))),
							TotalShares: sdk.NewDec(20),
						},
					},
					incentiveAccumulators: incentiveAccumsWithPoolId(2),
					incentiveRecords: []types.IncentiveRecord{
						{
							PoolId: uint64(2),
							IncentiveRecordBody: types.IncentiveRecordBody{
								RemainingCoin: sdk.NewDecCoin("bar", sdk.NewInt(5)),
								EmissionRate:  sdk.NewDec(10),
								StartTime:     defaultTime1,
							},
							MinUptime:   testUptimeOne,
							IncentiveId: 2,
						},
					},
				},
			}),
			expectedPools: []model.Pool{
				*poolOne,
				*poolTwo,
			},
			expectedTicksPerPoolId: map[uint64][]genesis.FullTick{
				1: {
					withTickIndex(defaultFullTick, -1234),
				},
				2: {
					withTickIndex(defaultFullTick, 0),
					withTickIndex(defaultFullTick, 999),
				},
			},
			expectedspreadFactorAccumValues: []genesis.AccumObject{
				{
					Name: types.KeySpreadRewardPoolAccumulator(1),
					AccumContent: &accum.AccumulatorContent{
						AccumValue:  sdk.NewDecCoins(sdk.NewDecCoin("foo", sdk.NewInt(10))),
						TotalShares: sdk.NewDec(10),
					},
				},
				{
					Name: types.KeySpreadRewardPoolAccumulator(2),
					AccumContent: &accum.AccumulatorContent{
						AccumValue:  sdk.NewDecCoins(sdk.NewDecCoin("bar", sdk.NewInt(20))),
						TotalShares: sdk.NewDec(20),
					},
				},
			},
			expectedIncentiveRecords: []types.IncentiveRecord{
				{
					PoolId: uint64(1),
					IncentiveRecordBody: types.IncentiveRecordBody{
						RemainingCoin: sdk.NewDecCoin("foo", sdk.NewInt(5)),
						EmissionRate:  sdk.NewDec(10),
						StartTime:     defaultTime1,
					},
					MinUptime: testUptimeOne,
				},
				{
					PoolId: uint64(2),
					IncentiveRecordBody: types.IncentiveRecordBody{
						RemainingCoin: sdk.NewDecCoin("bar", sdk.NewInt(5)),
						EmissionRate:  sdk.NewDec(10),
						StartTime:     defaultTime1,
					},
					MinUptime: testUptimeOne,
				},
			},
			expectedPositionData: []genesis.PositionData{
				{
					LockId:                  1,
					Position:                &testPositionModel,
					SpreadRewardAccumRecord: testSpreadRewardAccumRecord,
					UptimeAccumRecords:      testUptimeAccumRecord,
				},
				{
					LockId:                  2,
					Position:                withPositionId(testPositionModel, DefaultPositionId+1),
					SpreadRewardAccumRecord: testSpreadRewardAccumRecord,
					UptimeAccumRecords: []accum.Record{
						accumRecordWithDefinedValues(accumRecord, sdk.NewDec(99999), sdk.NewInt(10), sdk.NewInt(5)),
						accumRecordWithDefinedValues(accumRecord, sdk.NewDec(9999), sdk.NewInt(10), sdk.NewInt(5)),
						accumRecordWithDefinedValues(accumRecord, sdk.NewDec(999), sdk.NewInt(100), sdk.NewInt(50)),
						accumRecordWithDefinedValues(accumRecord, sdk.NewDec(99), sdk.NewInt(50), sdk.NewInt(25)),
						accumRecordWithDefinedValues(accumRecord, sdk.NewDec(9), sdk.NewInt(50), sdk.NewInt(25)),
						accumRecordWithDefinedValues(accumRecord, sdk.NewDec(9), sdk.NewInt(50), sdk.NewInt(25)),
					},
				},
			},
		},
	}

	for _, tc := range testCase {
		tc := tc

		s.Run(tc.name, func() {
			// This erases previously created pools.
			s.SetupTest()

			clKeeper := s.App.ConcentratedLiquidityKeeper
			ctx := s.Ctx

			clKeeper.InitGenesis(ctx, tc.genesis)

			// Check params
			clParamsAfterInitialization := clKeeper.GetParams(ctx)
			s.Require().Equal(tc.genesis.Params.String(), clParamsAfterInitialization.String())

			clPoolsAfterInitialization, err := clKeeper.GetPools(ctx)
			s.Require().NoError(err)

			// Check pools
			spreadFactorAccums := []accum.AccumulatorObject{}
			incentiveRecords := []types.IncentiveRecord{}
			s.Require().Equal(len(clPoolsAfterInitialization), len(tc.genesis.PoolData))
			for i, actualPoolI := range clPoolsAfterInitialization {
				actualPool, ok := actualPoolI.(*model.Pool)
				s.Require().True(ok)
				s.Require().Equal(tc.expectedPools[i], *actualPool)

				expectedTicks, ok := tc.expectedTicksPerPoolId[actualPool.Id]
				s.Require().True(ok)

				actualTicks, err := clKeeper.GetAllInitializedTicksForPoolWithoutPoolId(ctx, actualPool.Id)
				s.Require().NoError(err)

				// Validate ticks.
				s.validateTicksWithoutPoolId(expectedTicks, actualTicks)

				// get spread reward accumulator
				spreadFactorAccum, err := clKeeper.GetSpreadRewardAccumulator(s.Ctx, actualPool.GetId())
				s.Require().NoError(err)
				spreadFactorAccums = append(spreadFactorAccums, spreadFactorAccum)

				// check incentive accumulators
				acutalIncentiveAccums, err := clKeeper.GetUptimeAccumulators(ctx, actualPool.Id)
				s.Require().NoError(err)
				for j, actualIncentiveAccum := range acutalIncentiveAccums {
					expectedAccum := tc.genesis.PoolData[i].IncentivesAccumulators
					actualTotalShares, err := actualIncentiveAccum.GetTotalShares()
					s.Require().NoError(err)

					s.Require().Equal(expectedAccum[j].GetName(), actualIncentiveAccum.GetName())
					s.Require().Equal(expectedAccum[j].AccumContent.AccumValue, actualIncentiveAccum.GetValue())
					s.Require().Equal(expectedAccum[j].AccumContent.TotalShares, actualTotalShares)
				}

				// get incentive records for pool
				poolIncentiveRecords, err := clKeeper.GetAllIncentiveRecordsForPool(s.Ctx, actualPool.GetId())
				s.Require().NoError(err)
				incentiveRecords = append(incentiveRecords, poolIncentiveRecords...)
			}

			// get all positions.
			s.Require().NoError(err)
			var actualPositionData []genesis.PositionData
			for _, positionDataEntry := range tc.expectedPositionData {
				position, err := clKeeper.GetPosition(ctx, positionDataEntry.Position.PositionId)
				s.Require().NoError(err)

				actualLockId := uint64(0)
				if positionDataEntry.LockId != 0 {
					actualLockId, err = clKeeper.GetLockIdFromPositionId(ctx, positionDataEntry.Position.PositionId)
					s.Require().NoError(err)
				} else {
					_, err = clKeeper.GetLockIdFromPositionId(ctx, positionDataEntry.Position.PositionId)
					s.Require().Error(err)
					s.Require().ErrorIs(err, types.PositionIdToLockNotFoundError{PositionId: positionDataEntry.Position.PositionId})
				}
				positionWithoutPoolId := genesis.PositionWithoutPoolId{}
				positionWithoutPoolId.Address = position.Address
				positionWithoutPoolId.JoinTime = position.JoinTime
				positionWithoutPoolId.Liquidity = position.Liquidity
				positionWithoutPoolId.LowerTick = position.LowerTick
				positionWithoutPoolId.UpperTick = position.UpperTick
				positionWithoutPoolId.PositionId = position.PositionId

				actualPositionData = append(actualPositionData, genesis.PositionData{
					LockId:                  actualLockId,
					Position:                &positionWithoutPoolId,
					SpreadRewardAccumRecord: positionDataEntry.SpreadRewardAccumRecord,
					UptimeAccumRecords:      positionDataEntry.UptimeAccumRecords,
				})
			}

			// Validate positions
			s.Require().Equal(tc.expectedPositionData, actualPositionData)

			// Validate accum objects
			s.Require().Equal(len(spreadFactorAccums), len(tc.expectedspreadFactorAccumValues))
			for i, accumObject := range spreadFactorAccums {
				s.Require().Equal(spreadFactorAccums[i].GetValue(), tc.expectedspreadFactorAccumValues[i].AccumContent.AccumValue)

				totalShares, err := accumObject.GetTotalShares()
				s.Require().NoError(err)
				s.Require().Equal(totalShares, tc.expectedspreadFactorAccumValues[i].AccumContent.TotalShares)
			}

			// Validate incentive records
			s.Require().Equal(len(incentiveRecords), len(tc.expectedIncentiveRecords))
			for i, incentiveRecord := range incentiveRecords {
				s.Require().Equal(incentiveRecord.PoolId, tc.expectedIncentiveRecords[i].PoolId)
				s.Require().Equal(incentiveRecord.MinUptime, tc.expectedIncentiveRecords[i].MinUptime)
				s.Require().Equal(incentiveRecord.IncentiveRecordBody.EmissionRate.String(), tc.expectedIncentiveRecords[i].IncentiveRecordBody.EmissionRate.String())
				s.Require().Equal(incentiveRecord.IncentiveRecordBody.RemainingCoin.String(), tc.expectedIncentiveRecords[i].IncentiveRecordBody.RemainingCoin.String())
				s.Require().True(incentiveRecord.IncentiveRecordBody.StartTime.Equal(tc.expectedIncentiveRecords[i].IncentiveRecordBody.StartTime))
			}
			// Validate next position id.
			s.Require().Equal(tc.genesis.NextPositionId, clKeeper.GetNextPositionId(ctx))
		})
	}
}

// TestExportGenesis tests the ExportGenesis function of the ConcentratedLiquidityKeeper.
// It checks that the correct genesis state is returned.
func (s *KeeperTestSuite) TestExportGenesis() {
	s.SetupTest()

	poolE := s.PrepareConcentratedPool()
	poolOne, ok := poolE.(*model.Pool)
	s.Require().True(ok)

	poolE = s.PrepareConcentratedPool()
	poolTwo, ok := poolE.(*model.Pool)
	s.Require().True(ok)

	defaultTime1 := time.Unix(100, 100)
	defaultTime2 := time.Unix(300, 100)

	testCase := []struct {
		name    string
		genesis genesis.GenesisState
	}{
		{
			name: "one pool, one position, two ticks, one accumulator, two incentive records",
			genesis: setupGenesis(baseGenesis, []singlePoolGenesisEntry{
				{
					pool: *poolOne,
					tick: []genesis.FullTick{
						withTickIndex(defaultFullTickWithoutPoolId, -10),
						withTickIndex(defaultFullTickWithoutPoolId, 10),
					},
					positionData: []genesis.PositionData{
						{
							LockId:                  1,
							Position:                &testPositionModel,
							SpreadRewardAccumRecord: testSpreadRewardAccumRecord,
							UptimeAccumRecords:      testUptimeAccumRecord,
						},
					},
					spreadFactorAccumValues: genesis.AccumObject{
						Name: types.KeySpreadRewardPoolAccumulator(poolOne.Id),
						AccumContent: &accum.AccumulatorContent{
							AccumValue:  sdk.NewDecCoins(sdk.NewDecCoin("foo", sdk.NewInt(10))),
							TotalShares: sdk.NewDec(10),
						},
					},
					incentiveAccumulators: incentiveAccumsWithPoolId(1),
					incentiveRecords: []types.IncentiveRecord{
						{
							PoolId: uint64(1),
							IncentiveRecordBody: types.IncentiveRecordBody{
								RemainingCoin: sdk.NewDecCoin("bar", sdk.NewInt(15)),
								EmissionRate:  sdk.NewDec(20),
								StartTime:     defaultTime2,
							},
							MinUptime:   testUptimeOne,
							IncentiveId: 1,
						},
						{
							PoolId: uint64(1),
							IncentiveRecordBody: types.IncentiveRecordBody{
								RemainingCoin: sdk.NewDecCoin("foo", sdk.NewInt(5)),
								EmissionRate:  sdk.NewDec(10),
								StartTime:     defaultTime1,
							},
							MinUptime:   testUptimeOne,
							IncentiveId: 2,
						},
					},
				},
			}),
		},
		{
			name: "two pools, two positions, one tick pool one, two ticks pool two, two accumulators, one incentive records each",
			genesis: setupGenesis(baseGenesis, []singlePoolGenesisEntry{
				{
					pool: *poolOne,
					tick: []genesis.FullTick{
						withTickIndex(defaultFullTickWithoutPoolId, -1234),
					},
					positionData: []genesis.PositionData{
						{
							LockId:                  1,
							Position:                &testPositionModel,
							SpreadRewardAccumRecord: testSpreadRewardAccumRecord,
							UptimeAccumRecords:      testUptimeAccumRecord,
						},
						{
							LockId:                  0,
							Position:                withPositionId(testPositionModel, DefaultPositionId+1),
							SpreadRewardAccumRecord: testSpreadRewardAccumRecord,
							UptimeAccumRecords:      testUptimeAccumRecord,
						},
					},
					spreadFactorAccumValues: genesis.AccumObject{
						Name: types.KeySpreadRewardPoolAccumulator(poolOne.Id),
						AccumContent: &accum.AccumulatorContent{
							AccumValue:  sdk.NewDecCoins(sdk.NewDecCoin("foo", sdk.NewInt(10))),
							TotalShares: sdk.NewDec(10),
						},
					},
					incentiveAccumulators: incentiveAccumsWithPoolId(1),
					incentiveRecords: []types.IncentiveRecord{
						{
							PoolId: uint64(1),
							IncentiveRecordBody: types.IncentiveRecordBody{
								RemainingCoin: sdk.NewDecCoin("foo", sdk.NewInt(5)),
								EmissionRate:  sdk.NewDec(10),
								StartTime:     defaultTime1,
							},
							MinUptime: testUptimeOne,
						},
					},
				},
				{
					pool: *poolTwo,
					tick: []genesis.FullTick{
						withTickIndex(defaultFullTickWithoutPoolId, 0),
						withTickIndex(defaultFullTickWithoutPoolId, 9999),
					},
					spreadFactorAccumValues: genesis.AccumObject{
						Name: types.KeySpreadRewardPoolAccumulator(poolTwo.Id),
						AccumContent: &accum.AccumulatorContent{
							AccumValue:  sdk.NewDecCoins(sdk.NewDecCoin("bar", sdk.NewInt(20))),
							TotalShares: sdk.NewDec(20),
						},
					},
					incentiveAccumulators: incentiveAccumsWithPoolId(2),
					incentiveRecords: []types.IncentiveRecord{
						{
							PoolId: uint64(2),
							IncentiveRecordBody: types.IncentiveRecordBody{
								RemainingCoin: sdk.NewDecCoin("bar", sdk.NewInt(5)),
								EmissionRate:  sdk.NewDec(10),
								StartTime:     defaultTime1,
							},
							MinUptime: testUptimeOne,
						},
					},
					positionData: []genesis.PositionData{
						{
							LockId:                  2,
							Position:                withPositionId(testPositionModel, DefaultPositionId+2),
							SpreadRewardAccumRecord: testSpreadRewardAccumRecord,
							UptimeAccumRecords: []accum.Record{
								accumRecordWithDefinedValues(accumRecord, sdk.NewDec(99999), sdk.NewInt(10), sdk.NewInt(5)),
								accumRecordWithDefinedValues(accumRecord, sdk.NewDec(9999), sdk.NewInt(10), sdk.NewInt(5)),
								accumRecordWithDefinedValues(accumRecord, sdk.NewDec(999), sdk.NewInt(100), sdk.NewInt(50)),
								accumRecordWithDefinedValues(accumRecord, sdk.NewDec(99), sdk.NewInt(50), sdk.NewInt(25)),
								accumRecordWithDefinedValues(accumRecord, sdk.NewDec(9), sdk.NewInt(50), sdk.NewInt(25)),
								accumRecordWithDefinedValues(accumRecord, sdk.NewDec(9), sdk.NewInt(50), sdk.NewInt(25)),
							},
						},
					},
				},
			}),
		},
	}

	for _, tc := range testCase {
		tc := tc

		s.Run(tc.name, func() {
			s.SetupTest()

			clKeeper := s.App.ConcentratedLiquidityKeeper
			ctx := s.Ctx
			expectedGenesis := tc.genesis

			// System Under test
			clKeeper.InitGenesis(ctx, tc.genesis)

			// Export the genesis state.
			actualExported := clKeeper.ExportGenesis(ctx)

			// Validate params.
			s.Require().Equal(expectedGenesis.Params.String(), actualExported.Params.String())

			// Validate pools and ticks.
			s.Require().Equal(len(expectedGenesis.PoolData), len(actualExported.PoolData))
			for i, actualPoolData := range actualExported.PoolData {
				expectedPoolData := expectedGenesis.PoolData[i]
				s.Require().Equal(expectedPoolData.Pool, actualPoolData.Pool)

				s.validateTicksWithoutPoolId(expectedPoolData.Ticks, actualPoolData.Ticks)

				// validate spread reward accumulators
				s.Require().Equal(expectedPoolData.SpreadRewardAccumulator, actualPoolData.SpreadRewardAccumulator)

				// validate incentive accumulator
				for i, incentiveAccumulator := range actualPoolData.IncentivesAccumulators {
					s.Require().Equal(expectedPoolData.IncentivesAccumulators[i], incentiveAccumulator)
				}

				// Validate Incentive Records
				s.Require().Equal(len(expectedPoolData.IncentiveRecords), len(actualPoolData.IncentiveRecords))
				for i, incentiveRecord := range actualPoolData.IncentiveRecords {
					s.Require().Equal(incentiveRecord.PoolId, expectedPoolData.IncentiveRecords[i].PoolId)
					s.Require().Equal(incentiveRecord.MinUptime, expectedPoolData.IncentiveRecords[i].MinUptime)
					s.Require().Equal(incentiveRecord.IncentiveRecordBody.EmissionRate.String(), expectedPoolData.IncentiveRecords[i].IncentiveRecordBody.EmissionRate.String())
					s.Require().Equal(incentiveRecord.IncentiveRecordBody.RemainingCoin.String(), expectedPoolData.IncentiveRecords[i].IncentiveRecordBody.RemainingCoin.String())
					s.Require().True(incentiveRecord.IncentiveRecordBody.StartTime.Equal(expectedPoolData.IncentiveRecords[i].IncentiveRecordBody.StartTime))
				}

				// Validate positions.
				s.Require().Equal(expectedPoolData.PositionData, actualPoolData.PositionData)

				// Validate uptime accumulators
				for i, actualPositionData := range actualPoolData.PositionData {
					expectedPositionData := expectedPoolData.PositionData[i]
					// validate incentive accumulator
					for i, uptimeAccum := range actualPositionData.UptimeAccumRecords {
						s.Require().Equal(expectedPositionData.UptimeAccumRecords[i], uptimeAccum)
					}
				}
			}

			// Validate next position id.
			s.Require().Equal(tc.genesis.NextPositionId, actualExported.NextPositionId)
		})
	}
}

// TestMarshalUnmarshalGenesis tests the MarshalUnmarshalGenesis functions of the ConcentratedLiquidityKeeper.
// It checks that the exported genesis can be marshaled and unmarshaled without panicking.
func TestMarshalUnmarshalGenesis(t *testing.T) {
	// Set up the app and context
	app := osmoapp.Setup(false)
	ctx := app.BaseApp.NewContext(false, tmproto.Header{})
	now := ctx.BlockTime()
	ctx = ctx.WithBlockTime(now.Add(time.Second))

	// Create an app module for the ConcentratedLiquidityKeeper
	encodingConfig := osmoapp.MakeEncodingConfig()
	appCodec := encodingConfig.Marshaler
	appModule := clmodule.NewAppModule(appCodec, *app.ConcentratedLiquidityKeeper)

	// Export the genesis state
	genesisExported := appModule.ExportGenesis(ctx, appCodec)

	// Test that the exported genesis can be marshaled and unmarshaled without panicking
	assert.NotPanics(t, func() {
		app := osmoapp.Setup(false)
		ctx := app.BaseApp.NewContext(false, tmproto.Header{})
		ctx = ctx.WithBlockTime(now.Add(time.Second))
		am := clmodule.NewAppModule(appCodec, *app.ConcentratedLiquidityKeeper)
		am.InitGenesis(ctx, appCodec, genesisExported)
	})
}

func (s *KeeperTestSuite) TestParseFullTickFromBytes() {
	var (
		cdc = s.App.AppCodec()

		formatFullKey = func(tickPrefix []byte, poolIdBytes []byte, tickIndexBytes []byte) []byte {
			key := make([]byte, 0)
			key = append(key, tickPrefix...)
			key = append(key, poolIdBytes...)
			key = append(key, tickIndexBytes...)
			return key
		}
	)

	tests := map[string]struct {
		key           []byte
		val           []byte
		expectedValue genesis.FullTick
		expectedErr   error
	}{
		"valid positive tick": {
			key:           types.KeyTick(defaultPoolId, defaultTickIndex),
			val:           cdc.MustMarshal(&defaultTickInfo),
			expectedValue: defaultTickWithoutPoolId,
		},
		"valid zero tick": {
			key:           types.KeyTick(defaultPoolId, 0),
			val:           cdc.MustMarshal(&defaultTickInfo),
			expectedValue: withTickIndex(defaultTickWithoutPoolId, 0),
		},
		"valid negative tick": {
			key:           types.KeyTick(defaultPoolId, -1),
			val:           cdc.MustMarshal(&defaultTickInfo),
			expectedValue: withTickIndex(defaultTickWithoutPoolId, -1),
		},
		"valid negative tick large": {
			key:           types.KeyTick(defaultPoolId, -200),
			val:           cdc.MustMarshal(&defaultTickInfo),
			expectedValue: withTickIndex(defaultTickWithoutPoolId, -200),
		},
		"empty key": {
			key:         []byte{},
			val:         cdc.MustMarshal(&defaultTickInfo),
			expectedErr: types.ErrKeyNotFound,
		},
		"random key": {
			key: []byte{112, 12, 14, 4, 5},
			val: cdc.MustMarshal(&defaultTickInfo),
			expectedErr: types.InvalidTickKeyByteLengthError{
				Length: 5,
			},
		},
		"using not full key (wrong key)": {
			key: types.KeyTickPrefixByPoolId(defaultPoolId),
			val: cdc.MustMarshal(&defaultTickInfo),
			expectedErr: types.InvalidTickKeyByteLengthError{
				Length: len(types.TickPrefix) + cl.Uint64Bytes,
			},
		},
		"invalid prefix key": {
			key:         formatFullKey(types.PositionPrefix, sdk.Uint64ToBigEndian(defaultPoolId), types.TickIndexToBytes(defaultTickIndex)),
			val:         cdc.MustMarshal(&defaultTickInfo),
			expectedErr: types.InvalidPrefixError{Actual: string(types.PositionPrefix), Expected: string(types.TickPrefix)},
		},
		"invalid tick index encoding": {
			// must use types.TickIndexToBytes() on tick index for correct encoding.
			key: formatFullKey(types.TickPrefix, sdk.Uint64ToBigEndian(defaultPoolId), sdk.Uint64ToBigEndian(defaultTickIndex)),
			val: cdc.MustMarshal(&defaultTickInfo),
			expectedErr: types.InvalidTickKeyByteLengthError{
				Length: len(types.TickPrefix) + cl.Uint64Bytes + cl.Uint64Bytes,
			},
		},
		"invalid pool id encoding": {
			// format 1 byte.
			key: formatFullKey(types.TickPrefix, []byte(fmt.Sprintf("%x", defaultPoolId)), types.TickIndexToBytes(defaultTickIndex)),
			val: cdc.MustMarshal(&defaultTickInfo),
			expectedErr: types.InvalidTickKeyByteLengthError{
				Length: len(types.TickPrefix) + 2 + cl.Uint64Bytes,
			},
		},
	}

	for name, tc := range tests {
		tc := tc
		s.Run(name, func() {
			fullTick, err := cl.ParseFullTickFromBytes(tc.key, tc.val)
			if tc.expectedErr != nil {
				s.Require().Error(err)
				s.Require().ErrorIs(err, tc.expectedErr)
				s.Require().Equal(fullTick, genesis.FullTick{})
			} else {
				s.Require().NoError(err)

				// check result
				s.Require().Equal(tc.expectedValue, fullTick)
			}
		})
	}
}

func (s *KeeperTestSuite) TestGetAllInitializedTicksForPool() {
	const (
		// chosen randomly
		defaultPoolId = 676
	)

	tests := []struct {
		name                   string
		preSetTicks            []genesis.FullTick
		expectedTicksOverwrite []genesis.FullTick
		expectedError          error
	}{
		{
			name:        "one positive tick for pool",
			preSetTicks: []genesis.FullTick{defaultTick},
		},
		{
			name:        "one negative tick for pool",
			preSetTicks: []genesis.FullTick{withTickIndex(defaultTick, -1)},
		},
		{
			name:        "one zero tick for pool",
			preSetTicks: []genesis.FullTick{withTickIndex(defaultTick, 0)},
		},
		{
			name: "multiple ticks for pool",
			preSetTicks: []genesis.FullTick{
				defaultTick,
				withTickIndex(defaultTick, -1),
				withTickIndex(defaultTick, 0),
				withTickIndex(defaultTick, -200),
				withTickIndex(defaultTick, 1000),
				withTickIndex(defaultTick, -999),
			},
			expectedTicksOverwrite: []genesis.FullTick{
				withTickIndex(defaultTick, -999),
				withTickIndex(defaultTick, -200),
				withTickIndex(defaultTick, -1),
				withTickIndex(defaultTick, 0),
				defaultTick,
				withTickIndex(defaultTick, 1000),
			},
		},
	}

	for _, test := range tests {
		s.Run(test.name, func() {
			s.SetupTest()

			for _, tick := range test.preSetTicks {
				s.App.ConcentratedLiquidityKeeper.SetTickInfo(s.Ctx, defaultPoolId, tick.TickIndex, &tick.Info)
			}

			// If overwrite is not specified, we expect the pre-set ticks to be returned.
			expectedTicks := test.preSetTicks
			if len(test.expectedTicksOverwrite) > 0 {
				expectedTicks = test.expectedTicksOverwrite
			}

			// System Under Test
			ticks, err := s.App.ConcentratedLiquidityKeeper.GetAllInitializedTicksForPoolWithoutPoolId(s.Ctx, defaultPoolId)
			s.Require().NoError(err)

			s.Require().Equal(len(expectedTicks), len(ticks))
			s.validateTicksWithoutPoolId(expectedTicks, ticks)
		})
	}
}

func (s *KeeperTestSuite) validateTicksWithoutPoolId(expectedTicks []genesis.FullTick, actualTicks []genesis.FullTick) {
	s.Require().Equal(len(expectedTicks), len(actualTicks))
	for i, tick := range actualTicks {
		s.Require().Equal(expectedTicks[i].TickIndex, tick.TickIndex, "tick (%d) pool indexes are not equal", i)
		s.Require().Equal(expectedTicks[i].Info, tick.Info, "tick (%d) infos are not equal", i)
	}
}
