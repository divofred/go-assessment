package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.
// Code generated by github.com/99designs/gqlgen version v0.17.49

import (
	"context"

	"github.com/divofred/go-assessment/database"
	"github.com/divofred/go-assessment/graph/model"
)

// CreateStudentScore is the resolver for the createStudentScore field.
func (r *mutationResolver) CreateStudentScore(ctx context.Context, input []*model.StudentsScoreInput) ([]*model.StudentTotalScore, error) {
	return db.CreateStudentScore(input)
}

// GetSubjectAssessments is the resolver for the getSubjectAssessments field.
func (r *queryResolver) GetSubjectAssessments(ctx context.Context, subject string) (*model.SubjectAssessment, error) {
	return db.GetSubjectAssessments(subject), nil
}

// GetStudentAssessments is the resolver for the getStudentAssessments field.
func (r *queryResolver) GetStudentAssessments(ctx context.Context, name string) (*model.StudentOverallResult, error) {
	return db.GetStudentAssessments(name)
}

// Mutation returns MutationResolver implementation.
func (r *Resolver) Mutation() MutationResolver { return &mutationResolver{r} }

// Query returns QueryResolver implementation.
func (r *Resolver) Query() QueryResolver { return &queryResolver{r} }

type mutationResolver struct{ *Resolver }
type queryResolver struct{ *Resolver }

// !!! WARNING !!!
// The code below was going to be deleted when updating resolvers. It has been copied here so you have
// one last chance to move it out of harms way if you want. There are two reasons this happens:
//   - When renaming or deleting a resolver the old code will be put in here. You can safely delete
//     it when you're done.
//   - You have helper methods in this file. Move them out to keep these resolver files clean.
func (r *queryResolver) GetOverallPositions(ctx context.Context) ([]*model.StudentTotalScore, error) {
	return db.GetOverallPositions(), nil
}

var db = database.Connect()
