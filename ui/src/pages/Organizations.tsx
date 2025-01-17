import { Fragment } from "react";
import {
  Datagrid,
  List,
  TextField,
  Show,
  SimpleShowLayout,
  ReferenceField,
  BulkExportButton,
  BulkDeleteButton,
  ReferenceManyField,
  Create,
  SimpleForm,
  TextInput,
} from "react-admin";

const OrganizationListBulkActions = () => (
  <Fragment>
    <BulkExportButton />
    <BulkDeleteButton />
  </Fragment>
);

export const OrganizationList = () => (
  <List>
    <Datagrid
      rowClick="show"
      bulkActionButtons={<OrganizationListBulkActions />}
    >
      <TextField label="Name" source="name" />
      <TextField label="Description" source="description" />
      <ReferenceField
        label="Owner"
        source="owner_id"
        reference="users"
        link="show"
      />
    </Datagrid>
  </List>
);

export const OrganizationShow = () => (
  <Show>
    <SimpleShowLayout>
      <TextField label="ID" source="id" />
      <TextField label="Name" source="name" />
      <TextField label="Description" source="description" />
    </SimpleShowLayout>
  </Show>
);

export const OrganizationCreate = () => (
  <Create>
    <SimpleForm>
      <TextInput label="Name" source="name" />
      <TextInput label="Description" source="description" />
    </SimpleForm>
  </Create>
);
