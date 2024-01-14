package controllers

import (
	"fmt"
	"signalone/pkg/components"
	"signalone/pkg/models"
	"signalone/pkg/utils"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type LogAnalysisPayload struct {
	userId      string
	containerId string
	logs        string
}

type MainController struct {
	iEngine                 *utils.InferenceEngine
	applicationCollection   *mongo.Collection
	analysisStoreCollection *mongo.Collection
}

func NewMainController(iEngine *utils.InferenceEngine,
	applicationCollection *mongo.Collection,
	analysisStoreCollection *mongo.Collection) *MainController {
	return &MainController{
		iEngine:                 iEngine,
		applicationCollection:   applicationCollection,
		analysisStoreCollection: analysisStoreCollection,
	}
}

func (c *MainController) LogAnalysisTask(ctx *gin.Context) {
	var generatedSummary string
	var proposedSolutions components.SolutionPredictionResult
	var user models.User
	summarizationTaskPromptTemplate := `<|user|>
	Summarize these logs and generate a single paragraph summary of what is happening in these logs in high technical detail: %s</s>
	<|assistant|>`

	bearerToken := ctx.GetHeader("Authorization")
	if bearerToken == "" {
		ctx.JSON(401, gin.H{
			"message": "Unauthorized",
		})
		return
	}
	bearerToken = strings.TrimPrefix(bearerToken, "Bearer ")
	var logAnalysisPayload LogAnalysisPayload
	if err := ctx.ShouldBindJSON(&logAnalysisPayload); err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}
	userResult := c.applicationCollection.FindOne(ctx, bson.M{"id": logAnalysisPayload.userId})
	err := userResult.Decode(&user)
	if err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if user.AgentBearerToken != bearerToken {
		ctx.JSON(401, gin.H{
			"message": "Unauthorized",
		})
		return
	}
	issueId := uuid.New().String()
	generatedSummary = c.iEngine.LogSummarization(fmt.Sprintf(summarizationTaskPromptTemplate, logAnalysisPayload.logs))
	proposedSolutions = c.iEngine.PredictSolutions(generatedSummary)
	if !user.IsPro {
		c.analysisStoreCollection.InsertOne(ctx, models.SavedAnalysis{
			Logs:       logAnalysisPayload.logs,
			LogSummary: generatedSummary,
		})
	}

	c.applicationCollection.InsertOne(ctx, models.Issue{
		Id:                        issueId,
		UserId:                    logAnalysisPayload.userId,
		Logs:                      logAnalysisPayload.logs,
		LogSummary:                generatedSummary,
		PredictedSolutionsSummary: proposedSolutions.SolutionSummary,
		PredictedSolutionsSources: proposedSolutions.SolutionSources,
	})
	ctx.JSON(200, gin.H{
		"message": "Success",
		"issueId": issueId,
	})
}
