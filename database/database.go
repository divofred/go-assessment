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

func (db *DB) CreateStudentScore(studentsScores []*model.StudentsScoreInput) ([]*model.StudentTotalScore, error) {
	var studentScoreCollection = db.client.Database("go-assessment").Collection("studentScores")
	var isUploadSubjectCollection = db.client.Database("go-assessment").Collection("isUploadSubject")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for _, studentScoreInput := range studentsScores {
		// Check if the subject has already been uploaded
		filter := bson.M{"subject": studentScoreInput.Subject}
		var result bson.M
		err := isUploadSubjectCollection.FindOne(ctx, filter).Decode(&result)
		if err != nil && err != mongo.ErrNoDocuments {
			return nil, err
		}
		if err == nil {
			// Skip this subject since it has already been uploaded
			continue
		}

		// Sort students by score in descending order
		sort.SliceStable(studentScoreInput.Students, func(i, j int) bool {
			return studentScoreInput.Students[i].Score > studentScoreInput.Students[j].Score
		})

		var docs []interface{}
		for i, score := range studentScoreInput.Students {
			studentScore := &model.StudentScore{
				ID:       primitive.NewObjectID().Hex(),
				Name:     strings.ToTitle(score.Name),
				Subject:  studentScoreInput.Subject,
				Score:    score.Score,
				Position: i + 1,
			}
			docs = append(docs, studentScore)
		}

		// Insert student scores into the collection
		_, err = studentScoreCollection.InsertMany(ctx, docs)
		if err != nil {
			return nil, err
		}

		if _, err = isUploadSubjectCollection.InsertOne(ctx, bson.M{"subject": studentScoreInput.Subject}); err != nil {
			return nil, err
		}
	}

	cursor, err := studentScoreCollection.Find(ctx, bson.D{})
	if err != nil {
		log.Fatal(err)
	}
	defer cursor.Close(ctx)

	var studentScores []model.StudentScore

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

	return positions, nil
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

	for _, doc := range docs {
		filter := bson.M{"name": doc.(bson.M)["name"], "subject": doc.(bson.M)["subject"]}

		update := bson.M{
			"$set": doc,
		}

		_, err := studentTotalScoreCollection.UpdateOne(ctx, filter, update, options.Update().SetUpsert(true))
		if err != nil {
			log.Fatal(err)
		}
	}
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
		println("Total Scores: ", name, total)
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
