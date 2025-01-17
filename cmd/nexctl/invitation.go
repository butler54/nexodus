package main

import (
	"context"
	"fmt"
	"github.com/nexodus-io/nexodus/internal/api/public"
	"github.com/urfave/cli/v3"
)

func createInvitationCommand() *cli.Command {
	return &cli.Command{
		Name:  "invitation",
		Usage: "commands relating to invitations",
		Commands: []*cli.Command{
			{
				Name:  "list",
				Usage: "List invitations",
				Action: func(ctx context.Context, command *cli.Command) error {
					return listInvitations(ctx, command)
				},
			},
			{
				Name:  "create",
				Usage: "create an invitation",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "user-id",
						Required: false,
					},
					&cli.StringFlag{
						Name:     "email",
						Required: false,
					},
					&cli.StringFlag{
						Name:     "organization-id",
						Required: false,
					},
				},
				Action: func(ctx context.Context, command *cli.Command) error {
					organizationId, err := getUUID(command, "organization-id")
					if err != nil {
						return err
					}
					userId, err := getUUID(command, "user-id")
					if err != nil {
						return err
					}
					return createInvitation(ctx, command, public.ModelsAddInvitation{
						OrganizationId: organizationId,
						UserId:         userId,
						Email:          command.String("email"),
					})
				},
			},
			{
				Name:  "delete",
				Usage: "delete an invitation",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "inv-id",
						Required: true,
					},
				},
				Action: func(ctx context.Context, command *cli.Command) error {
					id, err := getUUID(command, "inv-id")
					if err != nil {
						return err
					}
					return deleteInvitation(ctx, command, id)
				},
			},
			{
				Name:  "accept",
				Usage: "accept an invitation",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "inv-id",
						Required: true,
					},
				},
				Action: func(ctx context.Context, command *cli.Command) error {
					id, err := getUUID(command, "inv-id")
					if err != nil {
						return err
					}
					return acceptInvitation(ctx, command, id)
				},
			},
		},
	}
}

func invitationsTableFields() []TableField {
	var fields []TableField
	fields = append(fields, TableField{Header: "INVITATION ID", Field: "Id"})
	fields = append(fields, TableField{Header: "ORGANIZATION", Formatter: func(item interface{}) string {
		inv := item.(public.ModelsInvitation)
		return inv.Organization.Name
	}})
	fields = append(fields, TableField{Header: "FROM", Formatter: func(item interface{}) string {
		inv := item.(public.ModelsInvitation)
		return fmt.Sprintf("%s <%s>", inv.From.FullName, inv.From.Username)
	}})
	fields = append(fields, TableField{Header: "EMAIL", Field: "Email"})
	fields = append(fields, TableField{Header: "EXPIRES AT", Field: "ExpiresAt"})
	return fields
}

func listInvitations(ctx context.Context, command *cli.Command) error {
	c := createClient(ctx, command)
	res := apiResponse(c.InvitationApi.
		ListInvitations(ctx).
		Execute())
	show(command, invitationsTableFields(), res)
	return nil
}
func acceptInvitation(ctx context.Context, command *cli.Command, id string) error {
	c := createClient(ctx, command)
	httpResp, err := c.InvitationApi.
		AcceptInvitation(ctx, id).
		Execute()
	_ = apiResponse("", httpResp, err)
	showSuccessfully(command, "accepted")
	return nil
}

func deleteInvitation(ctx context.Context, command *cli.Command, id string) error {
	c := createClient(ctx, command)
	res := apiResponse(c.InvitationApi.
		DeleteInvitation(ctx, id).
		Execute())
	show(command, invitationsTableFields(), res)
	showSuccessfully(command, "deleted")
	return nil
}

func createInvitation(ctx context.Context, command *cli.Command, invitation public.ModelsAddInvitation) error {
	c := createClient(ctx, command)
	if invitation.OrganizationId == "" {
		invitation.OrganizationId = getDefaultOrgId(ctx, c)
	}
	if invitation.UserId == "" && invitation.Email == "" {
		return fmt.Errorf("either the --user-id or --email flags are required")
	}
	res := apiResponse(c.InvitationApi.
		CreateInvitation(ctx).
		Invitation(invitation).
		Execute())
	show(command, invitationsTableFields(), res)
	return nil
}
