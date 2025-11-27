package qdrant

import (
	"armyknife/internal/vector"
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	pb "github.com/qdrant/go-client/qdrant"
	"golang.org/x/xerrors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Vec struct {
	collectionName string
	dimension      int
	conn           *grpc.ClientConn
}

func (s *Vec) parseOrCreateUUID(id string) string {
	uuidObj, err := uuid.Parse(id)
	if err != nil {
		uuidObj = uuid.NewSHA1(uuid.NameSpaceURL, []byte(id))
	}
	return uuidObj.String()
}

func ptrBool(b bool) *bool {
	return &b
}

func ptrUint32(u uint32) *uint32 {
	return &u
}

func NewVec(address string, collectionName string, dimension int) (*Vec, error) {
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, xerrors.Errorf("failed to connect to Qdrant: %w", err)
	}

	store := &Vec{
		collectionName: collectionName,
		dimension:      dimension,
		conn:           conn,
	}

	if err := store.initialize(); err != nil {
		conn.Close()
		return nil, xerrors.Errorf("failed to initialize: %w", err)
	}

	return store, nil
}

func (s *Vec) initialize() error {
	collectionsClient := pb.NewCollectionsClient(s.conn)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	listResp, err := collectionsClient.List(ctx, &pb.ListCollectionsRequest{})
	if err != nil {
		return xerrors.Errorf("failed to list collections: %w", err)
	}

	collectionExists := false
	for _, collection := range listResp.Collections {
		if collection.Name == s.collectionName {
			collectionExists = true
			break
		}
	}

	if !collectionExists {
		_, err = collectionsClient.Create(ctx, &pb.CreateCollection{
			CollectionName: s.collectionName,
			VectorsConfig: &pb.VectorsConfig{
				Config: &pb.VectorsConfig_Params{
					Params: &pb.VectorParams{
						Size:     uint64(s.dimension),
						Distance: pb.Distance_Cosine,
					},
				},
			},
		})
		if err != nil {
			return xerrors.Errorf("failed to create collection: %w", err)
		}
	}

	return nil
}

func (s *Vec) Index(ctx context.Context, id string, vector []float32, metadata map[string]string, source string) error {
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return xerrors.Errorf("failed to marshal metadata: %w", err)
	}

	payload := map[string]*pb.Value{
		"metadata": {
			Kind: &pb.Value_StringValue{StringValue: string(metadataJSON)},
		},
		"source": {
			Kind: &pb.Value_StringValue{StringValue: source},
		},
		"indexed_at": {
			Kind: &pb.Value_IntegerValue{IntegerValue: time.Now().Unix()},
		},
	}

	uuidStr := s.parseOrCreateUUID(id)

	points := []*pb.PointStruct{
		{
			Id: &pb.PointId{
				PointIdOptions: &pb.PointId_Uuid{Uuid: uuidStr},
			},
			Vectors: &pb.Vectors{
				VectorsOptions: &pb.Vectors_Vector{
					Vector: &pb.Vector{Data: vector},
				},
			},
			Payload: payload,
		},
	}

	client := pb.NewPointsClient(s.conn)
	_, err = client.Upsert(ctx, &pb.UpsertPoints{
		CollectionName: s.collectionName,
		Points:         points,
		Wait:           ptrBool(true),
	})
	if err != nil {
		return xerrors.Errorf("failed to upsert point: %w", err)
	}

	return nil
}

func (s *Vec) Search(ctx context.Context, queryVector []float32, limit int) ([]*vector.SearchResult, error) {
	client := pb.NewPointsClient(s.conn)
	searchResp, err := client.Search(ctx, &pb.SearchPoints{
		CollectionName: s.collectionName,
		Vector:         queryVector,
		Limit:          uint64(limit),
		WithPayload: &pb.WithPayloadSelector{
			SelectorOptions: &pb.WithPayloadSelector_Enable{Enable: true},
		},
	})
	if err != nil {
		return nil, xerrors.Errorf("failed to execute search: %w", err)
	}

	var results []*vector.SearchResult
	for _, scoredPoint := range searchResp.Result {
		var metadata map[string]string
		if metadataValue, ok := scoredPoint.Payload["metadata"]; ok {
			if metadataStr := metadataValue.GetStringValue(); metadataStr != "" {
				if err := json.Unmarshal([]byte(metadataStr), &metadata); err != nil {
					return nil, xerrors.Errorf("failed to unmarshal metadata: %w", err)
				}
			}
		}

		results = append(results, &vector.SearchResult{
			ID:         scoredPoint.Id.GetUuid(),
			Metadata:   metadata,
			Distance:   1.0 - float64(scoredPoint.Score),
			Similarity: float64(scoredPoint.Score),
		})
	}

	return results, nil
}

func (s *Vec) Exists(ctx context.Context, id string) (bool, error) {
	uuidStr := s.parseOrCreateUUID(id)

	client := pb.NewPointsClient(s.conn)
	retrieveResp, err := client.Get(ctx, &pb.GetPoints{
		CollectionName: s.collectionName,
		Ids: []*pb.PointId{
			{
				PointIdOptions: &pb.PointId_Uuid{Uuid: uuidStr},
			},
		},
	})
	if err != nil {
		return false, xerrors.Errorf("failed to check point existence: %w", err)
	}

	return len(retrieveResp.Result) > 0, nil
}

func (s *Vec) Delete(ctx context.Context, id string) error {
	uuidStr := s.parseOrCreateUUID(id)

	client := pb.NewPointsClient(s.conn)
	_, err := client.Delete(ctx, &pb.DeletePoints{
		CollectionName: s.collectionName,
		Points: &pb.PointsSelector{
			PointsSelectorOneOf: &pb.PointsSelector_Points{
				Points: &pb.PointsIdsList{
					Ids: []*pb.PointId{
						{
							PointIdOptions: &pb.PointId_Uuid{Uuid: uuidStr},
						},
					},
				},
			},
		},
		Wait: ptrBool(true),
	})
	if err != nil {
		return xerrors.Errorf("failed to delete point: %w", err)
	}

	return nil
}

func (s *Vec) DeleteOldEntries(ctx context.Context, source string, beforeTimestamp int64) error {
	const batchSize = 1000
	client := pb.NewPointsClient(s.conn)

	filter := &pb.Filter{
		Must: []*pb.Condition{
			{
				ConditionOneOf: &pb.Condition_Field{
					Field: &pb.FieldCondition{
						Key: "source",
						Match: &pb.Match{
							MatchValue: &pb.Match_Keyword{
								Keyword: source,
							},
						},
					},
				},
			},
			{
				ConditionOneOf: &pb.Condition_Field{
					Field: &pb.FieldCondition{
						Key: "indexed_at",
						Range: &pb.Range{
							Lt: &[]float64{float64(beforeTimestamp)}[0],
						},
					},
				},
			},
		},
	}

	scrollResp, err := client.Scroll(ctx, &pb.ScrollPoints{
		CollectionName: s.collectionName,
		Filter:         filter,
		Limit:          ptrUint32(100),
		WithPayload: &pb.WithPayloadSelector{
			SelectorOptions: &pb.WithPayloadSelector_Enable{Enable: false},
		},
	})
	if err != nil {
		return xerrors.Errorf("failed to scroll points: %w", err)
	}

	for {
		if len(scrollResp.Result) == 0 {
			break
		}

		var batch []*pb.PointId
		for _, point := range scrollResp.Result {
			batch = append(batch, point.Id)
			if len(batch) >= batchSize {
				_, err = client.Delete(ctx, &pb.DeletePoints{
					CollectionName: s.collectionName,
					Points: &pb.PointsSelector{
						PointsSelectorOneOf: &pb.PointsSelector_Points{
							Points: &pb.PointsIdsList{
								Ids: batch,
							},
						},
					},
					Wait: ptrBool(true),
				})
				if err != nil {
					return xerrors.Errorf("failed to delete batch: %w", err)
				}
				batch = batch[:0]
			}
		}

		if len(batch) > 0 {
			_, err = client.Delete(ctx, &pb.DeletePoints{
				CollectionName: s.collectionName,
				Points: &pb.PointsSelector{
					PointsSelectorOneOf: &pb.PointsSelector_Points{
						Points: &pb.PointsIdsList{
							Ids: batch,
						},
					},
				},
				Wait: ptrBool(true),
			})
			if err != nil {
				return xerrors.Errorf("failed to delete batch: %w", err)
			}
		}

		if scrollResp.NextPageOffset == nil {
			break
		}

		scrollResp, err = client.Scroll(ctx, &pb.ScrollPoints{
			CollectionName: s.collectionName,
			Filter:         filter,
			Limit:          ptrUint32(100),
			Offset:         scrollResp.NextPageOffset,
			WithPayload: &pb.WithPayloadSelector{
				SelectorOptions: &pb.WithPayloadSelector_Enable{Enable: false},
			},
		})
		if err != nil {
			return xerrors.Errorf("failed to scroll points: %w", err)
		}
	}

	return nil
}

func (s *Vec) Close() error {
	if s.conn != nil {
		return s.conn.Close()
	}
	return nil
}
