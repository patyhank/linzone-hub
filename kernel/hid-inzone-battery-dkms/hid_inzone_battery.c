// SPDX-License-Identifier: GPL-2.0
/*
 * Sony INZONE HID battery power_supply bridge.
 */

#include <linux/completion.h>
#include <linux/hid.h>
#include <linux/jiffies.h>
#include <linux/module.h>
#include <linux/mutex.h>
#include <linux/power_supply.h>
#include <linux/slab.h>

#define USB_VENDOR_ID_SONY			0x054c
#define USB_DEVICE_ID_SONY_INZONE_H9_WIRED	0x0e53
#define USB_DEVICE_ID_SONY_INZONE_H9_PS5	0x0e4c
#define USB_DEVICE_ID_SONY_INZONE_H9_DONGLE	0x0e61
#define USB_DEVICE_ID_SONY_INZONE_H7_3POLE	0x0dfd
#define USB_DEVICE_ID_SONY_INZONE_H7_USB	0x0e47
#define USB_DEVICE_ID_SONY_INZONE_H5		0x0ebf
#define USB_DEVICE_ID_SONY_INZONE_BUDS		0x0ec2
#define USB_DEVICE_ID_SONY_INZONE_BUDS_PS5	0x0ec3
#define USB_DEVICE_ID_SONY_INZONE_H10		0x0fa8
#define USB_DEVICE_ID_SONY_INZONE_E9_3POLE	0x0f80
#define USB_DEVICE_ID_SONY_INZONE_E9_4POLE	0x0f81
#define USB_DEVICE_ID_SONY_INZONE_H6_AIR_4POLE	0x0fc0
#define USB_DEVICE_ID_SONY_INZONE_H6_AIR_3POLE	0x0fc1
#define USB_DEVICE_ID_SONY_INZONE_MOUSE_A_WIRED	0x0fae
#define USB_DEVICE_ID_SONY_INZONE_MOUSE_A_DONGLE 0x0faf
#define USB_DEVICE_ID_SONY_INZONE_KBD_H75	0x0fb0

#define INZONE_HCI_REPORT_ID			0x02
#define INZONE_HCI_REPORT_SIZE			64
#define INZONE_KBM_REPORT_ID			0x00
#define INZONE_KBM_REPORT_SIZE			65
#define INZONE_KBM_CMD_GET			0xa0
#define INZONE_KBM_CMD_GET_CURRENT_BATTERY	0x04a0
#define INZONE_KBM_CMD_GET_CHARGE_STATUS	0x09a0
#define INZONE_HCI_CMD_PACKET_TYPE		0x01
#define INZONE_HCI_EVT_PACKET_TYPE		0x04
#define INZONE_HCI_EVT_CODE			0xff
#define INZONE_HCI_SONY_KEY_ID			0xc396
#define INZONE_HCI_VENDOR_OP			0xfc00
#define INZONE_HCI_ADDR_PC_RX			0x41
#define INZONE_HCI_FLAG_GET			0x01
#define INZONE_HCI_EVT_BATTERY_INFO		0x04

#define INZONE_CACHE_MS			30000
#define INZONE_RESPONSE_TIMEOUT_MS		1200

enum inzone_kind {
	INZONE_KIND_HEADSET,
	INZONE_KIND_BUDS,
	INZONE_KIND_KBM,
};

enum inzone_battery_slot {
	INZONE_SLOT_HEADSET,
	INZONE_SLOT_LEFT,
	INZONE_SLOT_RIGHT,
	INZONE_SLOT_CASE,
	INZONE_SLOT_COUNT,
};

struct inzone_battery;

struct inzone_power {
	struct inzone_battery *parent;
	struct power_supply *psy;
	struct power_supply_desc desc;
	enum inzone_battery_slot slot;
	char name[56];
	char model[64];
};

struct inzone_battery {
	struct hid_device *hdev;
	enum inzone_kind kind;
	struct mutex lock;
	struct completion response_ready;
	struct inzone_power supplies[INZONE_SLOT_COUNT];
	unsigned long last_update;
	u16 tx_id;
	u16 pending_kbm_cmd;
	bool present[INZONE_SLOT_COUNT];
	int capacity[INZONE_SLOT_COUNT];
	int capacity_level[INZONE_SLOT_COUNT];
	int status[INZONE_SLOT_COUNT];
};

static enum power_supply_property inzone_props[] = {
	POWER_SUPPLY_PROP_PRESENT,
	POWER_SUPPLY_PROP_STATUS,
	POWER_SUPPLY_PROP_TECHNOLOGY,
	POWER_SUPPLY_PROP_CAPACITY,
	POWER_SUPPLY_PROP_CAPACITY_LEVEL,
	POWER_SUPPLY_PROP_ENERGY_NOW,
	POWER_SUPPLY_PROP_ENERGY_FULL,
	POWER_SUPPLY_PROP_ENERGY_FULL_DESIGN,
	POWER_SUPPLY_PROP_POWER_NOW,
	POWER_SUPPLY_PROP_CHARGE_NOW,
	POWER_SUPPLY_PROP_CHARGE_FULL,
	POWER_SUPPLY_PROP_CHARGE_FULL_DESIGN,
	POWER_SUPPLY_PROP_SCOPE,
	POWER_SUPPLY_PROP_MODEL_NAME,
	POWER_SUPPLY_PROP_MANUFACTURER,
};

static int inzone_capacity_level(int capacity)
{
	if (capacity < 0)
		return POWER_SUPPLY_CAPACITY_LEVEL_UNKNOWN;
	if (capacity >= 95)
		return POWER_SUPPLY_CAPACITY_LEVEL_FULL;
	if (capacity >= 66)
		return POWER_SUPPLY_CAPACITY_LEVEL_HIGH;
	if (capacity >= 21)
		return POWER_SUPPLY_CAPACITY_LEVEL_NORMAL;
	return POWER_SUPPLY_CAPACITY_LEVEL_LOW;
}

static u8 inzone_hci_checksum(u8 address, u8 event_id, u8 event_type, u16 tx_id)
{
	u16 sum = 0;

	sum += INZONE_HCI_SONY_KEY_ID & 0xff;
	sum += INZONE_HCI_SONY_KEY_ID >> 8;
	sum += address;
	sum += event_id;
	sum += event_type;
	sum += tx_id & 0xff;
	sum += tx_id >> 8;

	return sum & 0xff;
}

static int inzone_send_hci_get(struct inzone_battery *bat, u8 event_id)
{
	u16 tx_id = ++bat->tx_id;
	u8 payload[] = {
		INZONE_HCI_CMD_PACKET_TYPE,
		INZONE_HCI_VENDOR_OP & 0xff, INZONE_HCI_VENDOR_OP >> 8,
		0x08,
		INZONE_HCI_SONY_KEY_ID & 0xff, INZONE_HCI_SONY_KEY_ID >> 8,
		INZONE_HCI_ADDR_PC_RX,
		event_id,
		INZONE_HCI_FLAG_GET,
		tx_id & 0xff, tx_id >> 8,
		0x00,
	};
	u8 report[INZONE_HCI_REPORT_SIZE] = {};
	int ret;

	payload[ARRAY_SIZE(payload) - 1] = inzone_hci_checksum(INZONE_HCI_ADDR_PC_RX,
							       event_id,
							       INZONE_HCI_FLAG_GET,
							       tx_id);
	reinit_completion(&bat->response_ready);

	report[0] = INZONE_HCI_REPORT_ID;
	report[1] = ARRAY_SIZE(payload);
	memcpy(&report[2], payload, sizeof(payload));

	ret = hid_hw_output_report(bat->hdev, report, sizeof(report));
	if (ret < 0)
		return ret;

	if (!wait_for_completion_timeout(&bat->response_ready,
					 msecs_to_jiffies(INZONE_RESPONSE_TIMEOUT_MS)))
		return -ETIMEDOUT;

	return 0;
}

static int inzone_send_kbm_get(struct inzone_battery *bat, u16 command)
{
	u8 report[INZONE_KBM_REPORT_SIZE] = {};
	int ret;

	reinit_completion(&bat->response_ready);
	bat->pending_kbm_cmd = command;

	report[0] = INZONE_KBM_REPORT_ID;
	report[1] = command & 0xff;
	report[2] = command >> 8;
	report[3] = 0x00;
	report[4] = 0x00;

	ret = hid_hw_output_report(bat->hdev, report, sizeof(report));
	if (ret < 0)
		return ret;

	if (!wait_for_completion_timeout(&bat->response_ready,
					 msecs_to_jiffies(INZONE_RESPONSE_TIMEOUT_MS)))
		return -ETIMEDOUT;

	return 0;
}

static void inzone_set_cell(struct inzone_battery *bat, enum inzone_battery_slot slot,
			    u8 status, u8 capacity)
{
	if (capacity > 100)
		return;

	bat->present[slot] = true;
	bat->capacity[slot] = capacity;
	bat->capacity_level[slot] = inzone_capacity_level(capacity);
	bat->status[slot] = status == 1 ? POWER_SUPPLY_STATUS_CHARGING :
					   POWER_SUPPLY_STATUS_DISCHARGING;
}

static int inzone_kbm_level_to_percent(u8 level)
{
	switch (level) {
	case 0:
		return 10;
	case 1:
		return 40;
	case 2:
		return 70;
	case 3:
		return 100;
	default:
		return -1;
	}
}

static void inzone_init_cell(struct inzone_battery *bat,
			     enum inzone_battery_slot slot)
{
	bat->present[slot] = false;
	if (bat->status[slot] == POWER_SUPPLY_STATUS_UNKNOWN)
		bat->status[slot] = POWER_SUPPLY_STATUS_DISCHARGING;
}

static void inzone_notify_supplies(struct inzone_battery *bat)
{
	int i;

	for (i = 0; i < INZONE_SLOT_COUNT; i++) {
		if (bat->supplies[i].psy)
			power_supply_changed(bat->supplies[i].psy);
	}
}

static int inzone_refresh_locked(struct inzone_battery *bat)
{
	int ret;
	bool was_present;
	int percent;

	if (time_before(jiffies, bat->last_update + msecs_to_jiffies(INZONE_CACHE_MS)))
		return 0;

	if (bat->kind == INZONE_KIND_KBM) {
		ret = inzone_send_kbm_get(bat, INZONE_KBM_CMD_GET_CURRENT_BATTERY);
		if (ret)
			return ret;

		was_present = bat->present[INZONE_SLOT_HEADSET];
		percent = bat->capacity[INZONE_SLOT_HEADSET];

		ret = inzone_send_kbm_get(bat, INZONE_KBM_CMD_GET_CHARGE_STATUS);
		if (ret == 0 && bat->present[INZONE_SLOT_HEADSET] && was_present)
			bat->capacity[INZONE_SLOT_HEADSET] = percent;

		bat->last_update = jiffies;
		return 0;
	}

	ret = inzone_send_hci_get(bat, INZONE_HCI_EVT_BATTERY_INFO);
	if (ret)
		return ret;

	bat->last_update = jiffies;
	return 0;
}

static int inzone_get_property(struct power_supply *psy,
			       enum power_supply_property psp,
			       union power_supply_propval *val)
{
	struct inzone_power *power = power_supply_get_drvdata(psy);
	struct inzone_battery *bat = power->parent;
	enum inzone_battery_slot slot = power->slot;

	mutex_lock(&bat->lock);
	switch (psp) {
	case POWER_SUPPLY_PROP_PRESENT:
		(void)inzone_refresh_locked(bat);
		val->intval = bat->present[slot];
		break;
	case POWER_SUPPLY_PROP_STATUS:
		(void)inzone_refresh_locked(bat);
		val->intval = bat->present[slot] ? bat->status[slot] :
						  POWER_SUPPLY_STATUS_UNKNOWN;
		break;
	case POWER_SUPPLY_PROP_TECHNOLOGY:
		val->intval = POWER_SUPPLY_TECHNOLOGY_LION;
		break;
	case POWER_SUPPLY_PROP_CAPACITY:
		(void)inzone_refresh_locked(bat);
		val->intval = bat->present[slot] ? bat->capacity[slot] : 0;
		break;
	case POWER_SUPPLY_PROP_CAPACITY_LEVEL:
		(void)inzone_refresh_locked(bat);
		val->intval = bat->present[slot] ? bat->capacity_level[slot] :
				    POWER_SUPPLY_CAPACITY_LEVEL_UNKNOWN;
		break;
	case POWER_SUPPLY_PROP_ENERGY_NOW:
		(void)inzone_refresh_locked(bat);
		val->intval = bat->present[slot] ? bat->capacity[slot] * 10000 : 0;
		break;
	case POWER_SUPPLY_PROP_ENERGY_FULL:
	case POWER_SUPPLY_PROP_ENERGY_FULL_DESIGN:
		val->intval = bat->present[slot] ? 1000000 : 0;
		break;
	case POWER_SUPPLY_PROP_POWER_NOW:
		val->intval = 0;
		break;
	case POWER_SUPPLY_PROP_CHARGE_NOW:
		(void)inzone_refresh_locked(bat);
		val->intval = bat->present[slot] ? bat->capacity[slot] : 0;
		break;
	case POWER_SUPPLY_PROP_CHARGE_FULL:
	case POWER_SUPPLY_PROP_CHARGE_FULL_DESIGN:
		val->intval = bat->present[slot] ? 100 : 0;
		break;
	case POWER_SUPPLY_PROP_SCOPE:
		val->intval = POWER_SUPPLY_SCOPE_DEVICE;
		break;
	case POWER_SUPPLY_PROP_MODEL_NAME:
		val->strval = power->model;
		break;
	case POWER_SUPPLY_PROP_MANUFACTURER:
		val->strval = "Sony";
		break;
	default:
		mutex_unlock(&bat->lock);
		return -EINVAL;
	}
	mutex_unlock(&bat->lock);

	return 0;
}

static int inzone_parse_battery_event(struct inzone_battery *bat, u8 *raw, int size)
{
	int start = -1;
	int i;
	u8 *data;
	int param_len;

	for (i = 0; i + 1 < size && i < 8; i++) {
		if (raw[i] == INZONE_HCI_EVT_PACKET_TYPE && raw[i + 1] == INZONE_HCI_EVT_CODE) {
			start = i;
			break;
		}
	}
	if (start < 0 || size < start + 13)
		return 0;

	data = raw + start;
	if (data[7] != INZONE_HCI_EVT_BATTERY_INFO)
		return 0;

	param_len = data[2] - 8;
	if (param_len < 2)
		return 0;

	if (bat->kind == INZONE_KIND_BUDS && param_len >= 6) {
		inzone_set_cell(bat, INZONE_SLOT_LEFT, data[11], data[12]);
		inzone_set_cell(bat, INZONE_SLOT_RIGHT, data[13], data[14]);
		inzone_set_cell(bat, INZONE_SLOT_CASE, data[15], data[16]);
		if (bat->supplies[INZONE_SLOT_LEFT].psy)
			power_supply_changed(bat->supplies[INZONE_SLOT_LEFT].psy);
		if (bat->supplies[INZONE_SLOT_RIGHT].psy)
			power_supply_changed(bat->supplies[INZONE_SLOT_RIGHT].psy);
		if (bat->supplies[INZONE_SLOT_CASE].psy)
			power_supply_changed(bat->supplies[INZONE_SLOT_CASE].psy);
	} else {
		inzone_set_cell(bat, INZONE_SLOT_HEADSET, data[11], data[12]);
		if (bat->supplies[INZONE_SLOT_HEADSET].psy)
			power_supply_changed(bat->supplies[INZONE_SLOT_HEADSET].psy);
	}

	complete(&bat->response_ready);
	return 1;
}

static int inzone_parse_kbm_event(struct inzone_battery *bat, u8 *raw, int size)
{
	u8 *data;
	u8 cmd;
	u8 index;
	u8 len;
	int percent;

	if (size >= 5 && raw[0] == INZONE_KBM_REPORT_ID) {
		data = raw + 1;
		size--;
	} else {
		data = raw;
	}

	if (size < 5)
		return 0;

	cmd = data[0];
	index = data[1];
	len = data[3];

	if (cmd != INZONE_KBM_CMD_GET)
		return 0;

	if (bat->pending_kbm_cmd != ((u16)index << 8 | cmd))
		return 0;

	if (bat->pending_kbm_cmd == INZONE_KBM_CMD_GET_CURRENT_BATTERY) {
		if (len < 1)
			return 0;
		percent = inzone_kbm_level_to_percent(data[4]);
		if (percent < 0)
			return 0;
		inzone_set_cell(bat, INZONE_SLOT_HEADSET, 0, percent);
		if (bat->supplies[INZONE_SLOT_HEADSET].psy)
			power_supply_changed(bat->supplies[INZONE_SLOT_HEADSET].psy);
		complete(&bat->response_ready);
		return 1;
	}

	if (bat->pending_kbm_cmd == INZONE_KBM_CMD_GET_CHARGE_STATUS) {
		if (len >= 1 && bat->present[INZONE_SLOT_HEADSET])
			bat->status[INZONE_SLOT_HEADSET] = data[4] == 1 ?
				POWER_SUPPLY_STATUS_CHARGING :
				POWER_SUPPLY_STATUS_DISCHARGING;
		complete(&bat->response_ready);
		return 1;
	}

	return 0;
}

static int inzone_raw_event(struct hid_device *hdev, struct hid_report *report,
			    u8 *data, int size)
{
	struct inzone_battery *bat = hid_get_drvdata(hdev);

	if (!bat)
		return 0;

	if (bat->kind == INZONE_KIND_KBM)
		return inzone_parse_kbm_event(bat, data, size);

	return inzone_parse_battery_event(bat, data, size);
}

static int inzone_register_supply(struct inzone_battery *bat,
				  enum inzone_battery_slot slot,
				  const char *suffix)
{
	struct power_supply_config cfg = {};
	struct inzone_power *power = &bat->supplies[slot];

	power->parent = bat;
	power->slot = slot;
	inzone_init_cell(bat, slot);
	snprintf(power->name, sizeof(power->name), "inzone_battery_%s_%04x_%04x",
		 suffix, bat->hdev->vendor, bat->hdev->product);
	if (bat->kind == INZONE_KIND_BUDS) {
		if (slot == INZONE_SLOT_LEFT)
			snprintf(power->model, sizeof(power->model), "INZONE Buds Left");
		else if (slot == INZONE_SLOT_RIGHT)
			snprintf(power->model, sizeof(power->model), "INZONE Buds Right");
		else if (slot == INZONE_SLOT_CASE)
			snprintf(power->model, sizeof(power->model), "INZONE Buds Case");
	} else if (bat->kind == INZONE_KIND_KBM) {
		snprintf(power->model, sizeof(power->model), "INZONE Keyboard/Mouse");
	} else {
		snprintf(power->model, sizeof(power->model), "INZONE Headset");
	}

	power->desc.name = power->name;
	power->desc.type = POWER_SUPPLY_TYPE_BATTERY;
	power->desc.properties = inzone_props;
	power->desc.num_properties = ARRAY_SIZE(inzone_props);
	power->desc.get_property = inzone_get_property;

	cfg.drv_data = power;
	power->psy = devm_power_supply_register(&bat->hdev->dev, &power->desc, &cfg);
	return PTR_ERR_OR_ZERO(power->psy);
}

static int inzone_probe(struct hid_device *hdev, const struct hid_device_id *id)
{
	struct inzone_battery *bat;
	int ret;
	int i;

	bat = devm_kzalloc(&hdev->dev, sizeof(*bat), GFP_KERNEL);
	if (!bat)
		return -ENOMEM;

	bat->hdev = hdev;
	bat->kind = id->driver_data;
	mutex_init(&bat->lock);
	init_completion(&bat->response_ready);
	for (i = 0; i < INZONE_SLOT_COUNT; i++) {
		bat->capacity[i] = -1;
		bat->capacity_level[i] = POWER_SUPPLY_CAPACITY_LEVEL_UNKNOWN;
		bat->status[i] = POWER_SUPPLY_STATUS_UNKNOWN;
	}

	hid_set_drvdata(hdev, bat);

	ret = hid_parse(hdev);
	if (ret)
		return ret;

	ret = hid_hw_start(hdev, HID_CONNECT_DEFAULT);
	if (ret)
		return ret;

	if (bat->kind == INZONE_KIND_BUDS) {
		ret = inzone_register_supply(bat, INZONE_SLOT_LEFT, "left");
		if (ret)
			goto err_stop;
		ret = inzone_register_supply(bat, INZONE_SLOT_RIGHT, "right");
		if (ret)
			goto err_stop;
		ret = inzone_register_supply(bat, INZONE_SLOT_CASE, "case");
		if (ret)
			goto err_stop;
	} else {
		ret = inzone_register_supply(bat, INZONE_SLOT_HEADSET, "headset");
		if (ret)
			goto err_stop;
	}

	mutex_lock(&bat->lock);
	(void)inzone_refresh_locked(bat);
	inzone_notify_supplies(bat);
	mutex_unlock(&bat->lock);

	hid_info(hdev, "registered INZONE battery power_supply\n");
	return 0;

err_stop:
	hid_hw_stop(hdev);
	return ret;
}

static void inzone_remove(struct hid_device *hdev)
{
	hid_hw_stop(hdev);
}

static const struct hid_device_id inzone_devices[] = {
	{ HID_USB_DEVICE(USB_VENDOR_ID_SONY, USB_DEVICE_ID_SONY_INZONE_H9_WIRED),
	  .driver_data = INZONE_KIND_HEADSET },
	{ HID_USB_DEVICE(USB_VENDOR_ID_SONY, USB_DEVICE_ID_SONY_INZONE_H9_PS5),
	  .driver_data = INZONE_KIND_HEADSET },
	{ HID_USB_DEVICE(USB_VENDOR_ID_SONY, USB_DEVICE_ID_SONY_INZONE_H9_DONGLE),
	  .driver_data = INZONE_KIND_HEADSET },
	{ HID_USB_DEVICE(USB_VENDOR_ID_SONY, USB_DEVICE_ID_SONY_INZONE_H7_3POLE),
	  .driver_data = INZONE_KIND_HEADSET },
	{ HID_USB_DEVICE(USB_VENDOR_ID_SONY, USB_DEVICE_ID_SONY_INZONE_H7_USB),
	  .driver_data = INZONE_KIND_HEADSET },
	{ HID_USB_DEVICE(USB_VENDOR_ID_SONY, USB_DEVICE_ID_SONY_INZONE_H5),
	  .driver_data = INZONE_KIND_HEADSET },
	{ HID_USB_DEVICE(USB_VENDOR_ID_SONY, USB_DEVICE_ID_SONY_INZONE_BUDS),
	  .driver_data = INZONE_KIND_BUDS },
	{ HID_USB_DEVICE(USB_VENDOR_ID_SONY, USB_DEVICE_ID_SONY_INZONE_BUDS_PS5),
	  .driver_data = INZONE_KIND_BUDS },
	{ HID_USB_DEVICE(USB_VENDOR_ID_SONY, USB_DEVICE_ID_SONY_INZONE_H10),
	  .driver_data = INZONE_KIND_HEADSET },
	{ HID_USB_DEVICE(USB_VENDOR_ID_SONY, USB_DEVICE_ID_SONY_INZONE_E9_3POLE),
	  .driver_data = INZONE_KIND_HEADSET },
	{ HID_USB_DEVICE(USB_VENDOR_ID_SONY, USB_DEVICE_ID_SONY_INZONE_E9_4POLE),
	  .driver_data = INZONE_KIND_HEADSET },
	{ HID_USB_DEVICE(USB_VENDOR_ID_SONY, USB_DEVICE_ID_SONY_INZONE_H6_AIR_4POLE),
	  .driver_data = INZONE_KIND_HEADSET },
	{ HID_USB_DEVICE(USB_VENDOR_ID_SONY, USB_DEVICE_ID_SONY_INZONE_H6_AIR_3POLE),
	  .driver_data = INZONE_KIND_HEADSET },
	{ HID_USB_DEVICE(USB_VENDOR_ID_SONY, USB_DEVICE_ID_SONY_INZONE_MOUSE_A_WIRED),
	  .driver_data = INZONE_KIND_KBM },
	{ HID_USB_DEVICE(USB_VENDOR_ID_SONY, USB_DEVICE_ID_SONY_INZONE_MOUSE_A_DONGLE),
	  .driver_data = INZONE_KIND_KBM },
	{ HID_USB_DEVICE(USB_VENDOR_ID_SONY, USB_DEVICE_ID_SONY_INZONE_KBD_H75),
	  .driver_data = INZONE_KIND_KBM },
	{ }
};
MODULE_DEVICE_TABLE(hid, inzone_devices);

static struct hid_driver inzone_driver = {
	.name = "hid-inzone-battery",
	.id_table = inzone_devices,
	.probe = inzone_probe,
	.remove = inzone_remove,
	.raw_event = inzone_raw_event,
};
module_hid_driver(inzone_driver);

MODULE_AUTHOR("LINZONE Hub contributors");
MODULE_DESCRIPTION("Sony INZONE HID battery power_supply driver");
MODULE_LICENSE("GPL");
