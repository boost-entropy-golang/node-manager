// Copyright 2019 dfuse Platform Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package nodeos

import (
	"context"
	"time"

	"github.com/dfuse-io/manageos/metrics"
	"go.uber.org/zap"
)

// Monitor manages the 'readinessProbe' bool for healthz purposes and
// the stateos drift/headblock.
//
// This should be performed through a go routine.
func (s *NodeosSuperviser) Monitor() {
	var lastHeadBlockTime time.Time
	var lastDbSizeTime time.Time

	getInfoFailureCount := 0

	for {
		time.Sleep(5 * time.Second)
		if !s.IsRunning() {
			getInfoFailureCount = 0
			continue
		}

		chainInfo, err := s.api.GetInfo(context.Background())
		if err != nil {
			zlog.Warn("got err on get into", zap.Error(err))
			getInfoFailureCount++
			if getInfoFailureCount > 5 {
				s.setReadinessProbeOff()
			}

			continue
		}

		zlog.Debug("Got chain info", zap.Duration("delta", time.Since(lastHeadBlockTime)))
		getInfoFailureCount = 0
		s.chainID = chainInfo.ChainID
		s.serverVersion = chainInfo.ServerVersion
		s.serverVersionString = chainInfo.ServerVersionString
		s.lastBlockSeen = uint32(chainInfo.HeadBlockNum)

		lastHeadBlockTime = chainInfo.HeadBlockTime.Time
		if s.options.MonitorHeadBlock {
			s.headBlockNumber.SetUint64(uint64(chainInfo.HeadBlockNum))
			s.headBlockTimeDrift.SetBlockTime(lastHeadBlockTime)
		}

		if s.options.ReadinessMaxLatency == 0 || time.Since(lastHeadBlockTime) < s.options.ReadinessMaxLatency {
			s.setReadinessProbeOn()
		}

		// monitor if BP is producer (should be 1 and only 1)
		if s.IsActiveProducer() {
			isProducerPaused, err := s.api.IsProducerPaused(context.Background())
			if err != nil {
				s.Logger.Debug("unable to check if producer is paused", zap.Error(err))
			} else {
				metrics.SetNodeosIsBlockProducer(isProducerPaused)
			}
		}

		if lastDbSizeTime.IsZero() || time.Now().Sub(lastDbSizeTime).Seconds() > 30.0 {
			s.Logger.Debug("first monitoring call or more than 30s has elapsed since last call, querying db size from nodeos")
			dbSize, err := s.api.GetDBSize(context.Background())
			if err != nil {
				s.Logger.Info("unable to get db size", zap.Error(err))
				continue
			}

			lastDbSizeTime = time.Now()

			metrics.NodeosDBSizeInfo.SetFloat64(float64(dbSize.FreeBytes), "FreeBytes")
			metrics.NodeosDBSizeInfo.SetFloat64(float64(dbSize.UsedBytes), "UsedBytes")
			metrics.NodeosDBSizeInfo.SetFloat64(float64(dbSize.Size), "Size")
		}
	}
}

func (s *NodeosSuperviser) setReadinessProbeOn() {
	if s.readinessProbe.CAS(false, true) {
		s.Logger.Info("nodeos superviser is now assumed to be ready")
	}
}

func (s *NodeosSuperviser) setReadinessProbeOff() {
	if s.readinessProbe.CAS(true, false) {
		s.Logger.Info("nodeos superviser is not ready anymore")
	}
}
