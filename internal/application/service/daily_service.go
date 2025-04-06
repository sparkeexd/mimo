package service

import (
	"fmt"
	"log"
	"strconv"

	"github.com/bwmarrin/discordgo"
	"github.com/go-co-op/gocron/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sparkeexd/mimo/internal/domain/action"
	"github.com/sparkeexd/mimo/internal/domain/network"
	"github.com/sparkeexd/mimo/internal/infrastructure/hoyolab"
	"github.com/sparkeexd/mimo/internal/infrastructure/postgres"
	"github.com/sparkeexd/mimo/pkg"
)

// Service that handles daily check-in commands.
type DailyService struct {
	DailyRepository hoyolab.DailyRepository
	TokenRepository postgres.TokenRepository
}

// Create a new daily service.
func NewDailyService(db *pgxpool.Pool) *DailyService {
	return &DailyService{
		DailyRepository: hoyolab.NewDailyRepository(),
		TokenRepository: postgres.NewTokenRepository(db),
	}
}

// Service's slash commands to be registered.
func (service *DailyService) Commands() map[string]action.Command {
	return map[string]action.Command{
		"daily": action.NewCommand(
			&discordgo.ApplicationCommand{
				Name:        "daily",
				Description: "Command for Genshin daily check-in.",
			},
			service.DailyClaimCommandHandler,
		),
	}
}

// Service's cron jobs to be registered.
func (service *DailyService) Jobs(session *discordgo.Session) []action.CronJob {
	return []action.CronJob{
		action.NewCronJob(
			gocron.CronJob("0 0 * * *", false),
			gocron.NewTask(service.AutoDailyClaimTaskHandler, session),
		),
	}
}

// Perform Genshin Impact daily check-in on HoYoLab.
func (service *DailyService) DailyClaimCommandHandler(session *discordgo.Session, interaction *discordgo.InteractionCreate) {
	session.InteractionRespond(interaction.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	discordUser := pkg.GetDiscordUser(interaction)
	userID, err := strconv.Atoi(discordUser.ID)
	if err != nil {
		content := "Invalid Discord user."
		pkg.InteractionResponseEditError(session, interaction.Interaction, err, content)
		return
	}

	token, err := service.TokenRepository.GetByUserID(userID)
	if err != nil {
		content := "You are not registered yet, please register first."
		pkg.InteractionResponseEditError(session, interaction.Interaction, err, content)
		return
	}

	cookie := network.NewCookie(token.LtokenV2, token.LtmidV2, token.LtuidV2)
	context := hoyolab.NewDailyRewardContext(hoyolab.Hk4eEndpoint, hoyolab.GenshinEventID, hoyolab.GenshinActID, hoyolab.GenshinSignGame)

	res, err := service.DailyRepository.Claim(cookie, context)
	message := fmt.Sprintf("You have successfully checked in, %s!", discordUser.Mention())
	if err != nil {
		message = fmt.Sprint(err)
	} else if res.Retcode != 0 {
		message = res.Message
	}

	session.InteractionResponseEdit(interaction.Interaction, &discordgo.WebhookEdit{
		Content: &message,
	})
}

// Task handler that automatically handles Genshin Impact daily check-in for all registered users.
func (service *DailyService) AutoDailyClaimTaskHandler(session *discordgo.Session) {
	startUserID := 0
	batchSize := 50

	tokens, err := service.TokenRepository.ListByBatch(startUserID, batchSize)
	if err != nil {
		log.Printf("Failed to list tokens: %v", err)
		return
	}

	for _, token := range tokens {
		cookie := network.NewCookie(token.LtokenV2, token.LtmidV2, token.LtuidV2)
		context := hoyolab.NewDailyRewardContext(hoyolab.Hk4eEndpoint, hoyolab.GenshinEventID, hoyolab.GenshinActID, hoyolab.GenshinSignGame)

		res, err := service.DailyRepository.Claim(cookie, context)
		content := res.Message
		if err != nil {
			content = "There was an issue with your daily check-in. Please try registering again."
		}

		channel, err := session.UserChannelCreate(strconv.Itoa(token.UserID))
		if err != nil {
			log.Printf("Failed to send message to user channel %d: %v", token.UserID, err)
			return
		}

		session.ChannelMessageSend(channel.ID, content)
	}
}
