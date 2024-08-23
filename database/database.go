package database

import (
	"context"
	"errors"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/divofred/go-assessment/graph/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var connectionString string = "mongodb://127.0.0.1:27017/assessment"

// DB Struct

type DB struct {
	client *mongo.Client
}

// Connect connects to the database.

func Connect() *DB {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(connectionString))
	if err != nil {
		log.Fatal(err)
	}

	return &DB{
		client: client,
	}
}

func (db *DB) CreateStudentScore(studentsScores model.StudentsScoreInput) ([]*model.StudentScore, error) {
	var studentScoreCollection = db.client.Database("go-assessment").Collection("studentScores")
	var isUploadSubjectCollection = db.client.Database("go-assessment").Collection("isUploadSubject")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	filter := bson.M{"subject": studentsScores.Subject}

	var result bson.M

	err := isUploadSubjectCollection.FindOne(ctx, filter).Decode(&result)
	if err != nil {
		if err != mongo.ErrNoDocuments {

			return nil, err
		}
	} else {
		return nil, errors.New("subject already uploaded")
	}

	sort.SliceStable(studentsScores.Students, func(i, j int) bool {
		return studentsScores.Students[i].Score > studentsScores.Students[j].Score
	})

	var docs []interface{}
	for i, score := range studentsScores.Students {
		studentScore := &model.StudentScore{
			ID:       primitive.NewObjectID().Hex(),
			Name:     strings.ToTitle(score.Name),
			Subject:  studentsScores.Subject,
			Score:    score.Score,
			Position: i + 1,
		}

		docs = append(docs, studentScore)
	}

	insertResult, err := studentScoreCollection.InsertMany(ctx, docs)
	if err != nil {
		log.Fatal((err))
	}

	var insertedScores []*model.StudentScore

	for i, id := range insertResult.InsertedIDs {
		insertedScores = append(insertedScores, &model.StudentScore{
			ID:       id.(primitive.ObjectID).Hex(),
			Name:     studentsScores.Students[i].Name,
			Subject:  studentsScores.Subject,
			Score:    studentsScores.Students[i].Score,
			Position: i + 1,
		})
	}

	if _, err = isUploadSubjectCollection.InsertOne(ctx, bson.M{"subject": studentsScores.Subject}); err != nil {
		log.Fatal(err)
	}

	return insertedScores, nil
}

func (db *DB) GetSubjectAssessments(subject string) *model.SubjectAssessment {
	var studentScoreCollection = db.client.Database("go-assessment").Collection("studentScores")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	filter := bson.M{"subject": strings.ToLower(subject)}

	findOptions := options.Find().SetSort(bson.D{{Key: "score", Value: -1}})

	cursor, err := studentScoreCollection.Find(ctx, filter, findOptions)
	if err != nil {
		log.Fatal(err)
	}
	defer cursor.Close(ctx)

	// Prepare a slice to hold the student scores
	var studentScores []*model.StudentScore

	// Iterate through the cursor and decode each document into a StudentScore
	for cursor.Next(ctx) {
		var score model.StudentScore
		if err := cursor.Decode(&score); err != nil {
			log.Fatal(err)
		}
		studentScores = append(studentScores, &score)
	}

	if err := cursor.Err(); err != nil {
		log.Fatal(err)
	}

	// Create the SubjectAssessment with the list of student scores
	subjectAssessment := &model.SubjectAssessment{
		Subject:  subject,
		Students: studentScores,
	}

	return subjectAssessment
}

func (db *DB) GetStudentAssessments(name string) (*model.StudentOverallResult, error) {
	var studentScoreCollection = db.client.Database("go-assessment").Collection("studentScores")
	var studentTotalScoreCollection = db.client.Database("go-assessment").Collection("studentTotalScores")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var totalScoreResult bson.M
	filter1 := bson.M{"name": strings.ToTitle(name)}

	err := studentTotalScoreCollection.FindOne(ctx, filter1).Decode(&totalScoreResult)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.New("student not found")
		}
		return nil, err
	}

	// Extract the position from the result
	position := int(totalScoreResult["position"].(int32))

	filter := bson.M{"name": strings.ToTitle(name)}

	cursor, err := studentScoreCollection.Find(ctx, filter)
	if err != nil {
		log.Fatal(err)
	}
	defer cursor.Close(ctx)

	var studentScores []*model.StudentScore

	// Iterate through the cursor and decode each document into a StudentScore
	for cursor.Next(ctx) {
		var score model.StudentScore
		if err := cursor.Decode(&score); err != nil {
			log.Fatal(err)
		}
		studentScores = append(studentScores, &score)
	}

	if err := cursor.Err(); err != nil {
		log.Fatal(err)
	}

	// Return the complete StudentOverallResult
	return &model.StudentOverallResult{
		Position: position,
		Result:   studentScores,
	}, nil
}

func (db *DB) GetOverallPositions() []*model.StudentTotalScore {
	var studentScoreCollection = db.client.Database("go-assessment").Collection("studentScores")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cursor, err := studentScoreCollection.Find(ctx, bson.D{})
	if err != nil {
		log.Fatal(err)
	}
	defer cursor.Close(ctx)

	var studentScores []model.StudentScore

	// Iterate through the cursor and decode each document into a StudentScore
	for cursor.Next(ctx) {
		var score model.StudentScore
		if err := cursor.Decode(&score); err != nil {
			log.Fatal(err)
		}
		studentScores = append(studentScores, score)
	}

	if err := cursor.Err(); err != nil {
		log.Fatal(err)
	}

	totalScores := aggregateScores(studentScores)
	sortedScores := sortByTotalScore(totalScores)
	positions := assignPositions(sortedScores)
	db.storeOverAllPositions(positions)

	return positions
}

func (db *DB) storeOverAllPositions(positions []*model.StudentTotalScore) {
	var studentTotalScoreCollection = db.client.Database("go-assessment").Collection("studentTotalScores")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var docs []interface{}
	for _, position := range positions {
		if position.Position == 0 {
			continue // Skip this record if position is zero or not set
		}

		doc := bson.M{
			"name":     position.Name,
			"total":    position.Total,
			"position": position.Position,
		}

		docs = append(docs, doc)
	}

	_, _ = studentTotalScoreCollection.InsertMany(ctx, docs)

}

func aggregateScores(scores []model.StudentScore) map[string]int {
	totalScores := make(map[string]int)

	for _, score := range scores {
		totalScores[score.Name] += score.Score
	}

	return totalScores
}

func sortByTotalScore(totalScores map[string]int) []*model.StudentTotalScore {
	var sortedScores []*model.StudentTotalScore

	for name, total := range totalScores {
		sortedScores = append(sortedScores, &model.StudentTotalScore{Name: name, Total: total})
	}

	sort.SliceStable(sortedScores, func(i, j int) bool {
		return sortedScores[i].Total > sortedScores[j].Total
	})

	return sortedScores
}

func assignPositions(sortedScores []*model.StudentTotalScore) []*model.StudentTotalScore {
	for i := range sortedScores {
		sortedScores[i].Position = i + 1
	}

	return sortedScores
}
