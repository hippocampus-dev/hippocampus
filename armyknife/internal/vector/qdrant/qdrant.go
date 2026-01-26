package qdrant

import (
	"armyknife/internal/ptr"
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
		Wait:           ptr.To(true),
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
		Wait: ptr.To(true),
	})
	if err != nil {
		return xerrors.Errorf("failed to delete point: %w", err)
	}

	return nil
}

func (s *Vec) Touch(ctx context.Context, ids []string) error {
	client := pb.NewPointsClient(s.conn)
	const batchSize = 500

	indexedAt := time.Now().Unix()
	payload := map[string]*pb.Value{
		"indexed_at": {
			Kind: &pb.Value_IntegerValue{IntegerValue: indexedAt},
		},
	}

	for i := 0; i < len(ids); i += batchSize {
		end := i + batchSize
		if end > len(ids) {
			end = len(ids)
		}
		batch := ids[i:end]

		batchPoints := make([]*pb.PointId, len(batch))
		for j, id := range batch {
			batchPoints[j] = &pb.PointId{
				PointIdOptions: &pb.PointId_Uuid{Uuid: s.parseOrCreateUUID(id)},
			}
		}

		_, err := client.SetPayload(ctx, &pb.SetPayloadPoints{
			CollectionName: s.collectionName,
			Payload:        payload,
			PointsSelector: &pb.PointsSelector{
				PointsSelectorOneOf: &pb.PointsSelector_Points{
					Points: &pb.PointsIdsList{
						Ids: batchPoints,
					},
				},
			},
			Wait: ptr.To(true),
		})
		if err != nil {
			return xerrors.Errorf("failed to touch batch: %w", err)
		}
	}

	return nil
}

func (s *Vec) DeleteOld(ctx context.Context, source string, olderThan int64) error {
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
							Lt: ptr.To(float64(olderThan)),
						},
					},
				},
			},
		},
	}

	_, err := client.Delete(ctx, &pb.DeletePoints{
		CollectionName: s.collectionName,
		Points: &pb.PointsSelector{
			PointsSelectorOneOf: &pb.PointsSelector_Filter{
				Filter: filter,
			},
		},
		Wait: ptr.To(true),
	})
	if err != nil {
		return xerrors.Errorf("failed to delete old points: %w", err)
	}

	return nil
}

func (s *Vec) Close() error {
	if s.conn != nil {
		return s.conn.Close()
	}
	return nil
}
